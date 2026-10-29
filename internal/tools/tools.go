package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/autokeren/payfox/internal/meta"
	"github.com/autokeren/payfox/internal/catalog"
	"github.com/autokeren/payfox/internal/commerce"
	"github.com/autokeren/payfox/internal/paypal"
	"github.com/autokeren/payfox/internal/mcp"
	"github.com/autokeren/payfox/internal/model"
)

// Publisher publishes a folder as a live static site and returns its public URL.
type Publisher interface {
	Publish(name string) (string, error)
}

type Registry struct {
	Workspace    string
	PublicBase   string
	Client       *meta.Client
	Publisher    Publisher
	OnImageSaved func(publicURL string)
	mcpServers   map[string]*mcp.Server
	mcpTools     map[string]*mcp.Server             // toolName -> owning server
	mcpDefs      map[string]model.ToolDefinition    // toolName -> full def (with parameters)
	PayPal       *paypal.Client
	Cat          *catalog.Catalog
	Store        *commerce.Store
	SendTemplateTo func(recipient string, payload map[string]any) error
	CurrentRecipient string
}

func New(workspace, publicBase string, client *meta.Client, pub Publisher) *Registry {
	return &Registry{
		Workspace: workspace, PublicBase: publicBase, Client: client, Publisher: pub,
		mcpServers: map[string]*mcp.Server{},
		mcpTools:   map[string]*mcp.Server{},
		mcpDefs:    map[string]model.ToolDefinition{},
	}
}

// SetCatalog attaches the product catalog.
func (r *Registry) SetCatalog(c *catalog.Catalog) { r.Cat = c }

// SetTemplateSender wires the Messenger template sender (product cards).
func (r *Registry) SetTemplateSender(f func(recipient string, payload map[string]any) error) { r.SendTemplateTo = f }

// SetStore attaches the commerce store (catalog + PayPal together).
func (r *Registry) SetStore(s *commerce.Store) { r.Store = s }

// SetPayPal attaches a PayPal API client for checkout operations.
func (r *Registry) SetPayPal(c *paypal.Client) { r.PayPal = c }

// AttachMCP connects to an MCP server and merges its tools into the registry.
// For browser servers (ghostfox), only KEY tools are exposed — too many tools
// confuse the model and it won't use any of them.
func (r *Registry) AttachMCP(ctx context.Context, srv *mcp.Server) error {
	if err := srv.Start(ctx); err != nil {
		return err
	}
	defs, err := srv.ListTools(ctx)
	if err != nil {
		return err
	}
	r.mcpServers[srv.Name] = srv

	// whitelist of browser tools worth exposing (prevents tool overload)
	browserWhitelist := map[string]bool{
		"session_create":  true,
		"page_open":       true,
		"page_screenshot": true,
		"page_snapshot":   true,
	}

	count := 0
	for _, d := range defs {
		name := d.Function.Name
		if srv.Name == "ghostfox" && !browserWhitelist[name] {
			continue // skip non-essential ghostfox tools
		}
		r.mcpTools[name] = srv
		r.mcpDefs[name] = d
		count++
	}
	fmt.Printf("  ⚡ MCP %s: %d tools (filtered from %d)\n", srv.Name, count, len(defs))
	return nil
}

// MCPToolCount returns how many MCP tools are attached.
func (r *Registry) MCPToolCount() int { return len(r.mcpTools) }

func (r *Registry) Definitions() []model.ToolDefinition {
	defs := r.nativeDefinitions()
	// append MCP tools (browser, etc.)
	for name := range r.mcpTools {
		if d := r.mcpDefinition(name); d != nil {
			defs = append(defs, *d)
		}
	}
	return defs
}

func (r *Registry) nativeDefinitions() []model.ToolDefinition {
	return []model.ToolDefinition{
		{Type: "function", Function: model.ToolFunction{
			Name: "write_file", Description: "Write a file into the workspace (websites, code, text). Use forward slashes. Directories are created automatically.",
			Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"relative path, e.g. sites/kopi/index.html"},"content":{"type":"string","description":"full file content"}},"required":["path","content"]}`),
		}},
		{Type: "function", Function: model.ToolFunction{
			Name: "edit_file", Description: "Edit a file by replacing one exact string with another. Much cheaper than rewriting a whole file — use this for small changes (change a title, a color, one section). old_string must match EXACTLY.",
			Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"file to edit"},"old_string":{"type":"string","description":"exact text to find"},"new_string":{"type":"string","description":"replacement text"}},"required":["path","old_string","new_string"]}`),
		}},
		{Type: "function", Function: model.ToolFunction{
			Name: "read_file", Description: "Read a file from the workspace.",
			Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`),
		}},
		{Type: "function", Function: model.ToolFunction{
			Name: "list_files", Description: "List files in the workspace.",
			Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"optional subfolder"}}}`),
		}},
		{Type: "function", Function: model.ToolFunction{
			Name: "gen_image", Description: "Generate an image with Muse Image (banners, logos, illustrations). Saved into workspace, returns its public URL.",
			Parameters: json.RawMessage(`{"type":"object","properties":{"prompt":{"type":"string","description":"what to draw, be specific"},"filename":{"type":"string","description":"output filename like banners/promo.png"}},"required":["prompt","filename"]}`),
		}},
		{Type: "function", Function: model.ToolFunction{
			Name: "publish_site", Description: "Publish a site folder as a LIVE public website and get its URL. Write the index.html first with write_file, then call this.",
			Parameters: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"site name (folder under sites/)"}},"required":["name"]}`),
		}},
		{Type: "function", Function: model.ToolFunction{
			Name: "send_photo", Description: "Send a local image FILE to the user directly in the chat (as a picture message). Use this to show screenshots of websites you verified.",
			Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"local file path of the image (e.g. from page_screenshot result)"}},"required":["path"]}`),
		}},
		{Type: "function", Function: model.ToolFunction{
			Name: "paypal_checkout", Description: "Create a PayPal checkout link for a product. Returns a payment URL the user can tap to pay. Use this AFTER the user confirms they want to buy.",
			Parameters: json.RawMessage(`{"type":"object","properties":{"item_name":{"type":"string","description":"product name e.g. Nike Air Max 42"},"amount":{"type":"string","description":"price as string e.g. 129.99"},"currency":{"type":"string","description":"3-letter currency code e.g. USD, IDR, EUR, SGD"},"description":{"type":"string","description":"optional short description"}},"required":["item_name","amount","currency"]}`),
		}},
		{Type: "function", Function: model.ToolFunction{
			Name: "paypal_invoice", Description: "Create a PayPal invoice (more formal than a checkout link). Returns an invoice ID and a payment URL. Use for B2B or multiple-item orders.",
			Parameters: json.RawMessage(`{"type":"object","properties":{"item_name":{"type":"string","description":"product or service name"},"amount":{"type":"string","description":"total price as string"},"currency":{"type":"string","description":"3-letter currency code"},"description":{"type":"string","description":"optional notes"}},"required":["item_name","amount","currency"]}`),
		}},
		{Type: "function", Function: model.ToolFunction{
			Name: "katalog",
			Description: "TAMPILKAN (SHOW) semua produk di katalog sebagai kartu produk cantik di chat. PAKAI INI ketika user bilang: 'katalog', 'tampilkan katalog', 'lihat produk', 'show catalog', 'browse', 'ada apa aja?', 'mau belanja'. JANGAN pakai ini untuk menambah produk!",
			Parameters: json.RawMessage(`{"type":"object","properties":{}}`),
		}},
		{Type: "function", Function: model.ToolFunction{
			Name: "tambah_produk",
			Description: "TAMBAH (ADD) produk BARU ke katalog toko. PAKAI INI HANYA ketika user bilang: 'tambah produk', 'add product', 'mau jual', 'masukin produk baru'. JANGAN pakai untuk menampilkan katalog!",
			Parameters: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"nama produk"},"price":{"type":"number","description":"harga"},"currency":{"type":"string","description":"kode mata uang: USD, IDR, EUR, SGD"},"image_url":{"type":"string","description":"URL foto produk (optional)"},"desc":{"type":"string","description":"deskripsi singkat (optional)"}},"required":["name","price","currency"]}`),
		}},
		{Type: "function", Function: model.ToolFunction{
			Name: "hapus_produk",
			Description: "HAPUS (REMOVE) produk dari katalog berdasarkan ID.",
			Parameters: json.RawMessage(`{"type":"object","properties":{"product_id":{"type":"string","description":"ID produk seperti prod_1"}},"required":["product_id"]}`),
		}},
		{Type: "function", Function: model.ToolFunction{
			Name: "bayar_paypal",
			Description: "BUAT LINK PEMBAYARAN PayPal untuk satu produk/order. PAKAI INI ketika user MAU BELI sesuatu dan sudah tahu produknya. Kembalikan link yang bisa di-tap untuk bayar.",
			Parameters: json.RawMessage(`{"type":"object","properties":{"product_id":{"type":"string","description":"ID produk dari katalog (prod_1, prod_2, dst). Kosongkan jika custom order."},"item_name":{"type":"string","description":"nama item jika custom (bukan dari katalog)"},"amount":{"type":"string","description":"harga sebagai string"},"currency":{"type":"string","description":"kode mata uang USD/IDR/EUR"}},"required":[]}`),
		}},
	}
}

func (r *Registry) mcpDefinition(name string) *model.ToolDefinition {
	if d, ok := r.mcpDefs[name]; ok {
		// prefix description with the MCP server name for context
		enriched := d
		enriched.Function.Description = fmt.Sprintf("[mcp:%s] %s", r.mcpTools[name].Name, d.Function.Description)
		return &enriched
	}
	return nil
}

func (r *Registry) Execute(ctx context.Context, call model.ToolCall) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("error: bad arguments: %v", err)
	}
	// MCP tools first (ghostfox browser, autokeren, etc.)
	if srv := r.mcpTools[call.Function.Name]; srv != nil {
		result, err := srv.CallTool(ctx, call.Function.Name, args)
		if err != nil {
			return "error: " + err.Error()
		}
		return result
	}

	switch call.Function.Name {
	case "write_file":
		return r.writeFile(str(args["path"]), str(args["content"]))
	case "edit_file":
		return r.editFile(str(args["path"]), str(args["old_string"]), str(args["new_string"]))
	case "read_file":
		return r.readFile(str(args["path"]))
	case "list_files":
		return r.listFiles(str(args["path"]))
	case "gen_image":
		return r.genImage(ctx, str(args["prompt"]), str(args["filename"]))
	case "publish_site":
		return r.publishSite(str(args["name"]))
	case "send_photo":
		return r.sendPhoto(str(args["path"]))
	case "paypal_checkout":
		if r.PayPal == nil {
			return "error: PayPal not configured — missing PAYPAL_CLIENT_ID / PAYPAL_CLIENT_SECRET"
		}
		name, _ := args["item_name"].(string)
		amount, _ := args["amount"].(string)
		currency, _ := args["currency"].(string)
		if currency == "" {
			currency = "USD"
		}
		desc, _ := args["description"].(string)
		retURL := r.PublicBase + "/paid"
		canURL := r.PublicBase
		link, err := r.PayPal.CreateCheckoutLink(name, amount, currency, retURL, canURL)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		_ = desc
		return fmt.Sprintf("✅ PayPal checkout created for '%s' (%s %s).\n\nPayment link: %s\n\nSend this link to the user so they can complete payment.", name, currency, amount, link)
	case "katalog":
		if r.Store == nil {
			return "error: store not loaded"
		}
		products := r.Store.Catalog.List()
		if len(products) == 0 {
			return "Katalog masih kosong. Pemilik toko bisa tambah produk dengan: tambah_produk"
		}
		// Send product cards to chat
		if r.SendTemplateTo != nil && r.CurrentRecipient != "" {
			tpl := r.Store.Template()
			if err := r.SendTemplateTo(r.CurrentRecipient, tpl); err != nil {
				// fallback to text
				return r.Store.ShowCatalog()
			}
			return fmt.Sprintf("✅ Katalog ditampilkan (%d produk) sebagai kartu produk.", len(products))
		}
		return r.Store.ShowCatalog()
	case "tambah_produk":
		if r.Store == nil {
			return "error: store not loaded"
		}
		name, _ := args["name"].(string)
		price, _ := args["price"].(float64)
		currency, _ := args["currency"].(string)
		if currency == "" {
			currency = "USD"
		}
		image, _ := args["image_url"].(string)
		desc, _ := args["desc"].(string)
		id, err := r.Store.AddProduct(name, price, currency, image, desc)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return fmt.Sprintf("✅ Produk '%s' (%s %.2f) ditambahkan ke katalog dengan ID %s. Customer bisa ketik 'katalog' untuk lihat semua produk.", name, currency, price, id)
	case "hapus_produk":
		if r.Store == nil {
			return "error: store not loaded"
		}
		id, _ := args["product_id"].(string)
		if err := r.Store.RemoveProduct(id); err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return fmt.Sprintf("✅ Produk %s dihapus dari katalog.", id)
	case "bayar_paypal":
		if r.Store == nil {
			return "error: store not loaded"
		}
		// If product_id given, buy from catalog
		if pid, ok := args["product_id"].(string); ok && pid != "" {
			link, err := r.Store.BuyProduct(pid)
			if err != nil {
				return fmt.Sprintf("error: %v", err)
			}
			p := r.Store.Catalog.Get(pid)
			return fmt.Sprintf("✅ Link pembayaran PayPal untuk %s (%s %.2f):\n\n%s\n\nKirim link ini ke user supaya bisa bayar.", p.Name, p.Currency, p.Price, link)
		}
		// Custom order
		name, _ := args["item_name"].(string)
		amount, _ := args["amount"].(string)
		currency, _ := args["currency"].(string)
		if currency == "" {
			currency = "USD"
		}
		if name == "" || amount == "" {
			return "error: butuh item_name dan amount untuk custom order"
		}
		if r.PayPal == nil {
			return "error: PayPal not configured"
		}
		link, err := r.PayPal.CreateCheckoutLink(name, amount, currency, r.PublicBase+"/paid", r.PublicBase)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return fmt.Sprintf("✅ Link pembayaran PayPal untuk %s (%s %s):\n\n%s", name, currency, amount, link)
	case "paypal_invoice":
		if r.PayPal == nil {
			return "error: PayPal not configured — missing PAYPAL_CLIENT_ID / PAYPAL_CLIENT_SECRET"
		}
		name, _ := args["item_name"].(string)
		amount, _ := args["amount"].(string)
		currency, _ := args["currency"].(string)
		if currency == "" {
			currency = "USD"
		}
		desc, _ := args["description"].(string)
		if desc == "" {
			desc = "Order via PayFox AI Agent"
		}
		invID, err := r.PayPal.CreateInvoice(name, desc, amount, currency)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		payLink, err := r.PayPal.SendInvoice(invID)
		if err == nil && payLink != "" {
			return fmt.Sprintf("✅ PayPal invoice %s created for '%s' (%s %s).\n\nPayment link: %s", invID, name, currency, amount, payLink)
		}
		return fmt.Sprintf("✅ PayPal invoice created (ID: %s) for '%s' (%s %s). Invoice is in DRAFT status — the user can pay via PayPal business dashboard.", invID, name, currency, amount)
	default:
		return fmt.Sprintf("error: unknown tool %s", call.Function.Name)
	}
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

var safePath = regexp.MustCompile(`^[a-zA-Z0-9._\-/]+$`)

func (r *Registry) resolve(rel string) (string, error) {
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || !safePath.MatchString(rel) || strings.Contains(rel, "..") {
		return "", fmt.Errorf("invalid path: %q", rel)
	}
	return filepath.Join(r.Workspace, rel), nil
}

func (r *Registry) writeFile(rel, content string) string {
	full, err := r.resolve(rel)
	if err != nil {
		return "error: " + err.Error()
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "error: " + err.Error()
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return "error: " + err.Error()
	}
	return fmt.Sprintf("written %s (%d bytes)", rel, len(content))
}

func (r *Registry) editFile(rel, oldStr, newStr string) string {
	full, err := r.resolve(rel)
	if err != nil {
		return "error: " + err.Error()
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "error: " + err.Error()
	}
	content := string(data)
	if !strings.Contains(content, oldStr) {
		return fmt.Sprintf("error: old_string not found in %s — read the file first and copy the exact text", rel)
	}
	count := strings.Count(content, oldStr)
	if count > 1 {
		return fmt.Sprintf("error: old_string appears %d times in %s — provide a longer unique string", count, rel)
	}
	updated := strings.Replace(content, oldStr, newStr, 1)
	if err := os.WriteFile(full, []byte(updated), 0o644); err != nil {
		return "error: " + err.Error()
	}
	return fmt.Sprintf("edited %s: replaced %d chars → %d chars", rel, len(oldStr), len(newStr))
}

func (r *Registry) readFile(rel string) string {
	full, err := r.resolve(rel)
	if err != nil {
		return "error: " + err.Error()
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "error: " + err.Error()
	}
	return string(data)
}

func (r *Registry) listFiles(rel string) string {
	base, err := r.resolve(rel)
	if err != nil {
		return "error: " + err.Error()
	}
	if rel == "" {
		base = r.Workspace
	}
	var files []string
	_ = filepath.Walk(base, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel2, _ := filepath.Rel(r.Workspace, p)
			files = append(files, rel2)
		}
		return nil
	})
	if len(files) == 0 {
		return "(empty)"
	}
	return strings.Join(files, "\n")
}

func (r *Registry) genImage(ctx context.Context, prompt, filename string) string {
	if !strings.Contains(filename, "/") {
		filename = "images/" + filename
	}
	full, err := r.resolve(filename)
	if err != nil {
		return "error: " + err.Error()
	}
	if !strings.HasSuffix(full, ".png") && !strings.HasSuffix(full, ".jpg") {
		full += ".png"
	}
	res, err := r.Client.GenerateImage(ctx, prompt, "1024x1024")
	if err != nil {
		return "error: " + err.Error()
	}
	var data []byte
	if res.B64JSON != "" {
		if b, err := base64.StdEncoding.DecodeString(res.B64JSON); err == nil {
			data = b
		}
	}
	if len(data) == 0 && res.URL != "" {
		return fmt.Sprintf("image generated at %s — tell the user the link (and save via download if supported)", res.URL)
	}
	if len(data) == 0 {
		return "error: no image data in response"
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "error: " + err.Error()
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return "error: " + err.Error()
	}
	publicURL := r.PublicBase + "/images/" + strings.TrimPrefix(filename, "images/")
	if r.OnImageSaved != nil {
		r.OnImageSaved(publicURL)
	}
	return fmt.Sprintf("image generated and SENT to the user as a picture message. URL: %s (no need to repeat the URL in your reply — just confirm what you made)", publicURL)
}

func (r *Registry) sendPhoto(path string) string {
	// absolute or workspace-relative
	full := path
	if !strings.HasPrefix(path, "/") {
		full = filepath.Join(r.Workspace, path)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "error: " + err.Error()
	}
	ext := ".png"
	if strings.HasSuffix(full, ".jpg") || strings.HasSuffix(full, ".jpeg") {
		ext = ".jpg"
	}
	os.MkdirAll(filepath.Join(r.Workspace, "images"), 0o755)
	name := filepath.Base(full)
	if name == "" {
		name = "photo" + ext
	}
	dest := filepath.Join(r.Workspace, "images", name)
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "error: " + err.Error()
	}
	publicURL := r.PublicBase + "/images/" + name
	if r.OnImageSaved != nil {
		r.OnImageSaved(publicURL)
	}
	return fmt.Sprintf("photo sent to the user's chat as a picture message: %s", publicURL)
}

func (r *Registry) publishSite(name string) string {
	if r.Publisher == nil {
		return "error: publisher not configured"
	}
	if name == "" {
		return "error: site name required"
	}
	url, err := r.Publisher.Publish(name)
	if err != nil {
		return "error: " + err.Error()
	}
	return fmt.Sprintf("site is LIVE at %s", url)
}

