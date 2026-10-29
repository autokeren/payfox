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
}

func New(cat *catalog.Catalog, pp *paypal.Client, publicURL string) *Store {
	return &Store{Catalog: cat, PayPal: pp, PublicURL: publicURL}
}

// ShowCatalog returns a text summary (for the AI to read).
func (s *Store) ShowCatalog() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	products := s.Catalog.List()
	if len(products) == 0 {
		return "Katalog masih kosong. Pemilik toko bisa tambah produk dengan: add_product"
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
		return "", fmt.Errorf("produk %s tidak ada di katalog. Ketik 'katalog' untuk lihat produk", productID)
	}
	if s.PayPal == nil {
		return "", fmt.Errorf("PayPal belum terkonfigurasi")
	}
	link, err := s.PayPal.CreateCheckoutLink(p.Name, fmt.Sprintf("%.2f", p.Price), p.Currency, s.PublicURL+"/paid", s.PublicURL)
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
