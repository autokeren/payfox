package messenger

import (
	"strings"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Event is a single incoming message extracted from a webhook payload.
type Event struct {
	SenderID string
	RecipientID string
	Text     string
	Postback   string // postback payload (e.g. "BUY:prod_1") — set when user taps a button
	Attachments []Attachment
}

type Attachment struct {
	Type      string `json:"type"`
	PayloadURL string `json:"payload_url"`
}

type webhookPayload struct {
	Entry []struct {
		Messaging []struct {
			Sender    struct{ ID string `json:"id"` } `json:"sender"`
			Recipient struct{ ID string `json:"id"` } `json:"recipient"`
			Postback  struct {
				Title   string `json:"title"`
				Payload string `json:"payload"`
			} `json:"postback"`
			Message   struct {
				MID        string `json:"mid"`
				Text       string `json:"text"`
				Attachments []struct {
					Type    string `json:"type"`
					Payload struct {
						URL string `json:"url"`
					} `json:"payload"`
				} `json:"attachments"`
			} `json:"message"`
		} `json:"messaging"`
	} `json:"entry"`
}

// ParseWebhook decodes the Messenger webhook body into simple events.
func ParseWebhook(body io.Reader) ([]Event, error) {
	var p webhookPayload
	if err := json.NewDecoder(body).Decode(&p); err != nil {
		return nil, err
	}
	var out []Event
	for _, entry := range p.Entry {
		for _, m := range entry.Messaging {
			ev := Event{
				SenderID:    m.Sender.ID,
				RecipientID: m.Recipient.ID,
				Text:       m.Message.Text,
				Postback:   m.Postback.Payload,
			}
			// Postback (button tap) → convert to a text-like event
			if ev.Postback != "" {
				if strings.HasPrefix(ev.Postback, "BUY:") {
					prodID := strings.TrimPrefix(ev.Postback, "BUY:")
					ev.Text = fmt.Sprintf("[BUY_BUTTON:%s]", prodID)
				}
				out = append(out, ev)
				continue
			}
			for _, a := range m.Message.Attachments {
				ev.Attachments = append(ev.Attachments, Attachment{Type: a.Type, PayloadURL: a.Payload.URL})
			}
			if ev.Text == "" && len(ev.Attachments) == 0 {
				continue
			}
			out = append(out, ev)
		}
	}
	return out, nil
}

// Sender sends text replies via the Messenger Send API.
type Sender struct {
	token string
	http  *http.Client
}

func NewSender(token string) *Sender {
	return &Sender{token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

func (s *Sender) Send(recipient, text string) error {
	if s.token == "" {
		return fmt.Errorf("no page access token")
	}
	payload, _ := json.Marshal(map[string]any{
		"recipient": map[string]any{"id": recipient},
		"message":   map[string]any{"text": text},
	})
	req, err := http.NewRequest(http.MethodPost, "https://graph.facebook.com/v21.0/me/messages", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	q := req.URL.Query()
	q.Set("access_token", s.token)
	req.URL.RawQuery = q.Encode()
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Action sends a sender_action (typing_on / typing_off / mark_seen).
func (s *Sender) Action(recipient, action string) error {
	if s.token == "" {
		return fmt.Errorf("no page access token")
	}
	payload, _ := json.Marshal(map[string]any{
		"recipient":    map[string]any{"id": recipient},
		"sender_action": action,
	})
	req, err := http.NewRequest(http.MethodPost, "https://graph.facebook.com/v21.0/me/messages", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	q := req.URL.Query()
	q.Set("access_token", s.token)
	req.URL.RawQuery = q.Encode()
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// SendImage sends a native image attachment (Facebook fetches the URL once
// and stores it — displays as a regular image bubble in the chat).
func (s *Sender) SendImage(recipient, imageURL string) error {
	if s.token == "" {
		return fmt.Errorf("no page access token")
	}
	payload, _ := json.Marshal(map[string]any{
		"recipient": map[string]any{"id": recipient},
		"message": map[string]any{
			"attachment": map[string]any{
				"type": "image",
				"payload": map[string]any{
					"url":        imageURL,
					"is_reusable": true,
				},
			},
		},
	})
	req, err := http.NewRequest(http.MethodPost, "https://graph.facebook.com/v21.0/me/messages", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	q := req.URL.Query()
	q.Set("access_token", s.token)
	req.URL.RawQuery = q.Encode()
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send image %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	return nil
}

// DownloadAttachment fetches an attachment payload URL (Facebook CDN) and
// returns the raw bytes. Audio (voice notes) arrive as .mp4/.m4a; images as .jpg/.png.
func DownloadAttachment(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// SendTemplate sends a structured template message (product cards, buttons, etc.)
// payload example:
// {"message":{"attachment":{"type":"template","payload":{"template_type":"generic","elements":[...]}}}}
func (s *Sender) SendTemplate(recipient string, templatePayload map[string]any) error {
	if s.token == "" {
		return fmt.Errorf("no page access token")
	}
	body := map[string]any{
		"messaging_type": "RESPONSE",
		"recipient":      map[string]any{"id": recipient},
	}
	for k, v := range templatePayload {
		body[k] = v
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, "https://graph.facebook.com/v21.0/me/messages?access_token="+s.token, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respRaw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("send template %d: %s", resp.StatusCode, string(respRaw))
	}
	return nil
}

// GetUserName fetches the customer's first/last name via the User Profile API.
func (s *Sender) GetUserName(userID string) (first, last string) {
	url := fmt.Sprintf("https://graph.facebook.com/v21.0/%s?fields=first_name,last_name&access_token=%s", userID, s.token)
	resp, err := s.http.Get(url)
	if err != nil {
		return "Customer", ""
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if json.Unmarshal(raw, &out) == nil && out.FirstName != "" {
		return out.FirstName, out.LastName
	}
	return "Customer", ""
}
