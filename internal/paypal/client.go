package paypal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	SandboxBase = "https://api-m.sandbox.paypal.com"
	LiveBase    = "https://api-m.paypal.com"
)

type Client struct {
	ClientID     string
	ClientSecret string
	BaseURL      string
	HTTP         *http.Client
	AccessToken  string
	TokenExpiry  time.Time
}

func New(clientID, clientSecret string, sandbox bool) *Client {
	base := LiveBase
	if sandbox {
		base = SandboxBase
	}
	return &Client{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		BaseURL:      base,
		HTTP:         &http.Client{Timeout: 30 * time.Second},
	}
}

// getToken gets an OAuth2 access token
func (c *Client) getToken() error {
	if c.AccessToken != "" && time.Now().Before(c.TokenExpiry) {
		return nil
	}
	data := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/v1/oauth2/token", bytes.NewBufferString(data.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.ClientID, c.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("paypal auth %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	c.AccessToken = out.AccessToken
	c.TokenExpiry = time.Now().Add(time.Duration(out.ExpiresIn-60) * time.Second)
	return nil
}

// CreateInvoice creates a PayPal invoice
func (c *Client) CreateInvoice(itemName, description string, amount string, currency string) (string, error) {
	if err := c.getToken(); err != nil {
		return "", err
	}
	invoice := map[string]any{
		"detail": map[string]any{
			"currency_code": currency,
			"note":          description,
		},
		"items": []map[string]any{
			{
				"name":        itemName,
				"quantity":     "1",
				"unit_amount": map[string]any{
					"currency_code": currency,
					"value":         amount,
				},
			},
		},
		"additional_recipients": []map[string]any{},
	}
	body, _ := json.Marshal(invoice)
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/v2/invoicing/invoices", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("paypal invoice %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// SendInvoice sends the invoice and gets the payment link
func (c *Client) SendInvoice(invoiceID string) (string, error) {
	if err := c.getToken(); err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/v2/invoicing/invoices/"+invoiceID+"/send", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("paypal send %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		Href string `json:"href"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		// Sometimes the link is in a different field
		var out2 struct {
			Links []struct {
				Href   string `json:"href"`
				Rel    string `json:"rel"`
			} `json:"links"`
		}
		if err2 := json.Unmarshal(raw, &out2); err2 == nil {
			for _, l := range out2.Links {
				if l.Rel == "payment_url" || l.Rel == "view" {
					return l.Href, nil
				}
			}
		}
		return "", fmt.Errorf("paypal send: no payment link found")
	}
	return out.Href, nil
}

// CreateCheckoutLink creates a PayPal checkout payment link (simpler than invoice)
func (c *Client) CreateCheckoutLink(itemName, amount, currency, returnURL, cancelURL string) (string, string, error) {
	if err := c.getToken(); err != nil {
		return "", "", err
	}
	order := map[string]any{
		"intent": "CAPTURE",
		"purchase_units": []map[string]any{
			{
				"amount": map[string]any{
					"currency_code": currency,
					"value":         amount,
				},
				"description": itemName,
			},
		},
		"application_context": map[string]any{
			"return_url": returnURL,
			"cancel_url":  cancelURL,
		},
	}
	body, _ := json.Marshal(order)
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/v2/checkout/orders", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", "", fmt.Errorf("paypal order %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		ID    string `json:"id"`
		Links []struct {
			Href string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"links"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", "", err
	}
	// Find the approve/payer link
	for _, l := range out.Links {
		if l.Rel == "approve" || l.Rel == "payer-action" {
			return l.Href, out.ID, nil
		}
	}
	return "", out.ID, fmt.Errorf("paypal order: no approve link found")
}

// Verify checks the credentials work
func (c *Client) Verify() error {
	return c.getToken()
}

// CaptureOrder captures an approved order — moves the money.
// Must be called AFTER the customer approves the payment.
func (c *Client) CaptureOrder(orderID string) error {
	if err := c.getToken(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/v2/checkout/orders/"+orderID+"/capture", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PayPal-Request-Id", "capture-"+orderID)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("paypal capture %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}
