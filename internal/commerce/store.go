package commerce

import (
	"fmt"
	"strings"
	"sync"

	"github.com/autokeren/payfox/internal/catalog"
	"github.com/autokeren/payfox/internal/paypal"
)

// Store ties together the catalog and PayPal — the single source of truth for commerce.
type Store struct {
	mu      sync.RWMutex
	Catalog *catalog.Catalog
	PayPal  *paypal.Client
	PublicURL string
	Tracker *OrderTracker
}

func New(cat *catalog.Catalog, pp *paypal.Client, publicURL string, tracker *OrderTracker) *Store {
	return &Store{Catalog: cat, PayPal: pp, PublicURL: publicURL, Tracker: tracker}
}

// ShowCatalog returns a text summary (for the AI to read).
func (s *Store) ShowCatalog() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	products := s.Catalog.List()
	if len(products) == 0 {
		return "The catalog is empty. The store owner can add products with tambah_produk."
	}
	var sb strings.Builder
	sb.WriteString("KATALOG TOKO:\n\n")
	for _, p := range products {
		sb.WriteString(fmt.Sprintf("• [%s] %s — %s %.2f\n", p.ID, p.Name, p.Currency, p.Price))
		if p.Desc != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", p.Desc))
		}
	}
	return sb.String()
}

// BuyProduct creates a PayPal checkout for a product by ID.
// Returns the payment link.
func (s *Store) BuyProduct(productID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.Catalog.Get(productID)
	if p == nil {
		return "", fmt.Errorf("Product %s not found in catalog. Type 'catalog' to see available products.", productID)
	}
	if s.PayPal == nil {
		return "", fmt.Errorf("PayPal is not configured")
	}
	link, _, err := s.PayPal.CreateCheckoutLink(p.Name, fmt.Sprintf("%.2f", p.Price), p.Currency, s.PublicURL+"/paid", s.PublicURL)
	if err != nil {
		return "", err
	}
	return link, nil
}

// AddProduct adds to catalog.
func (s *Store) AddProduct(name string, price float64, currency, image, desc string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := catalog.Product{Name: name, Price: price, Currency: currency, Image: image, Desc: desc}
	if err := s.Catalog.Add(p); err != nil {
		return "", err
	}
	return p.ID, nil
}

// RemoveProduct removes from catalog.
func (s *Store) RemoveProduct(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Catalog.Remove(id)
}

// Template builds the Messenger product cards payload.
func (s *Store) Template() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Catalog.MessengerGenericTemplate()
}

// CheckoutTracked creates a PayPal checkout AND registers the order in the
// tracker so the webhook can find the user when PayPal confirms payment.
func (s *Store) CheckoutTracked(productID, userID string) (*Order, string, error) {
	p := s.Catalog.Get(productID)
	if p == nil {
		return nil, "", fmt.Errorf("produk %s tidak ada di katalog", productID)
	}
	if s.PayPal == nil {
		return nil, "", fmt.Errorf("PayPal is not configured")
	}
	amount := fmt.Sprintf("%.2f", p.Price)
	link, orderID, err := s.PayPal.CreateCheckoutLink(p.Name, amount, p.Currency, s.PublicURL+"/paid", s.PublicURL)
	if err != nil {
		return nil, "", err
	}
	o := &Order{
		OrderID:     orderID,
		UserID:      userID,
		ProductID:   productID,
		ProductName: p.Name,
		Amount:      amount,
		Currency:    p.Currency,
		Status:      "CREATED",
	}
	if s.Tracker != nil {
		s.Tracker.Track(o)
	}
	return o, link, nil
}

// HandleWebhookEvent processes a PayPal webhook payload. Returns the order if
// it maps to a known Messenger user (so the server can notify them in chat).
func (s *Store) HandleWebhookEvent(eventType, orderID string) *Order {
	if s.Tracker == nil || orderID == "" {
		return nil
	}
	switch eventType {
	case "CHECKOUT.ORDER.APPROVED":
		return s.Tracker.MarkPaid(orderID, "APPROVED")
	case "PAYMENT.CAPTURE.COMPLETED", "CHECKOUT.ORDER.COMPLETED":
		return s.Tracker.MarkPaid(orderID, "COMPLETED")
	}
	return nil
}

// ReceiptTemplate builds a Messenger RECEIPT card for a completed order.
// This is the beautiful confirmation the customer sees after paying.
func (s *Store) ReceiptTemplate(o *Order, firstName, lastName string) map[string]any {
	p := s.Catalog.Get(o.ProductID)
	element := map[string]any{
		"title":     o.ProductName,
		"quantity":  1,
		"price":     o.Amount,
		"currency":  o.Currency,
		"subtitle":  "PayFox Store",
	}
	if p != nil && p.Image != "" {
		element["image_url"] = p.Image
	}

	recipientName := firstName
	if lastName != "" {
		recipientName += " " + lastName
	}
	if recipientName == "" {
		recipientName = "PayFox Customer"
	}

	amount := 0.0
	fmt.Sscanf(o.Amount, "%f", &amount)

	return map[string]any{
		"message": map[string]any{
			"attachment": map[string]any{
				"type": "template",
				"payload": map[string]any{
					"template_type":  "receipt",
					"recipient_name":  recipientName,
					"order_number":    o.OrderID,
					"currency":        o.Currency,
					"payment_method":  "PayPal",
					"timestamp":       fmt.Sprintf("%d", o.PaidAt.Unix()),
					"elements":        []map[string]any{element},
					"summary": map[string]any{
						"subtotal":    amount,
						"total_cost": amount,
					},
				},
			},
		},
	}
}
