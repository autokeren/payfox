package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Product struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
	Image    string  `json:"image,omitempty"`
	Desc     string  `json:"desc,omitempty"`
	URL      string  `json:"url,omitempty"`
}

type Catalog struct {
	mu       sync.RWMutex
	filePath string
	Products []Product `json:"products"`
}

func Load(workspace string) (*Catalog, error) {
	fp := filepath.Join(workspace, "catalog.json")
	c := &Catalog{filePath: fp}
	raw, err := os.ReadFile(fp)
	if err != nil {
		if os.IsNotExist(err) {
			c.Products = []Product{}
			c.save()
			return c, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(raw, c); err != nil {
		return nil, err
	}
	c.filePath = fp
	return c, nil
}

func (c *Catalog) save() error {
	raw, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(c.filePath, raw, 0644)
}

func (c *Catalog) Add(p Product) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if p.ID == "" {
		p.ID = fmt.Sprintf("prod_%d", len(c.Products)+1)
	}
	for _, e := range c.Products {
		if e.ID == p.ID {
			return fmt.Errorf("product %s already exists", p.ID)
		}
	}
	c.Products = append(c.Products, p)
	return c.save()
}

func (c *Catalog) Remove(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, p := range c.Products {
		if p.ID == id {
			c.Products = append(c.Products[:i], c.Products[i+1:]...)
			return c.save()
		}
	}
	return fmt.Errorf("product %s not found", id)
}

func (c *Catalog) Get(id string) *Product {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, p := range c.Products {
		if p.ID == id {
			return &p
		}
	}
	return nil
}

func (c *Catalog) List() []Product {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Products
}

func FormatPrice(p Product) string {
	return fmt.Sprintf("%s %.2f", p.Currency, p.Price)
}

func (c *Catalog) MessengerGenericTemplate() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	elements := []map[string]any{}
	for _, p := range c.Products {
		el := map[string]any{
			"title":    p.Name,
			"subtitle": fmt.Sprintf("%s — %s", FormatPrice(p), p.Desc),
			"buttons": []map[string]any{
				{
					"type":    "postback",
					"title":   "🛒 Buy Now",
					"payload": "BUY:" + p.ID,
				},
			},
		}
		if p.Image != "" {
			el["image_url"] = p.Image
		}
		elements = append(elements, el)
	}

	return map[string]any{
		"message": map[string]any{
			"attachment": map[string]any{
				"type": "template",
				"payload": map[string]any{
					"template_type": "generic",
					"elements":      elements,
				},
			},
		},
	}
}
