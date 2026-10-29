package commerce

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Order tracks a PayPal order → Messenger user mapping so the
// PayPal webhook can find who to notify when payment completes.
type Order struct {
	OrderID     string    `json:"order_id"`
	UserID      string    `json:"user_id"`      // Messenger user who tapped Buy
	ProductID   string    `json:"product_id"`
	ProductName string    `json:"product_name"`
	Amount      string    `json:"amount"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status"` // CREATED / APPROVED / COMPLETED
	CreatedAt   time.Time `json:"created_at"`
	PaidAt      time.Time `json:"paid_at,omitempty"`
}

// OrderTracker persists orders to disk so webhooks survive restarts.
type OrderTracker struct {
	mu       sync.RWMutex
	filePath string
	Orders   map[string]*Order `json:"orders"`
}

func NewOrderTracker(workspace string) (*OrderTracker, error) {
	fp := filepath.Join(workspace, "orders.json")
	t := &OrderTracker{filePath: fp, Orders: map[string]*Order{}}
	raw, err := os.ReadFile(fp)
	if err == nil {
		_ = json.Unmarshal(raw, t)
	}
	t.filePath = fp
	return t, nil
}

func (t *OrderTracker) save() {
	raw, _ := json.MarshalIndent(t, "", "  ")
	_ = os.WriteFile(t.filePath, raw, 0644)
}

// Track stores a new order.
func (t *OrderTracker) Track(o *Order) {
	t.mu.Lock()
	defer t.mu.Unlock()
	o.CreatedAt = time.Now()
	t.Orders[o.OrderID] = o
	t.save()
}

// Find returns the order by PayPal order ID.
func (t *OrderTracker) Find(orderID string) *Order {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if o, ok := t.Orders[orderID]; ok {
		return o
	}
	return nil
}

// MarkPaid sets status + sends notification through the callback.
func (t *OrderTracker) MarkPaid(orderID, status string) *Order {
	t.mu.Lock()
	defer t.mu.Unlock()
	o, ok := t.Orders[orderID]
	if !ok {
		return nil
	}
	o.Status = status
	o.PaidAt = time.Now()
	t.save()
	return o
}

// AllOrders returns every order (for status tool).
func (t *OrderTracker) AllOrders() []*Order {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]*Order, 0, len(t.Orders))
	for _, o := range t.Orders {
		out = append(out, o)
	}
	return out
}
