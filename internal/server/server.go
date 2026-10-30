package server

import (
	"encoding/json"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/autokeren/payfox/internal/agent"
	"github.com/autokeren/payfox/internal/commerce"
	"github.com/autokeren/payfox/internal/channels/messenger"
	"github.com/autokeren/payfox/internal/meta"
	"github.com/autokeren/payfox/internal/model"
	"github.com/autokeren/payfox/internal/config"
	"github.com/autokeren/payfox/internal/sitefs"
)

type Server struct {
	Cfg    *config.Config
	Port   int
	Sites  *sitefs.Store
	msg    *messenger.Sender
	agents map[string]*agent.Agent
	mu     sync.Mutex
	http   *http.Server
	Store        *commerce.Store       // catalog + PayPal together
	Tracker      *commerce.OrderTracker // order tracking for PayPal webhooks
	welcomed     map[string]bool        // users who got the welcome message
}

func New(cfg *config.Config, sites *sitefs.Store) *Server {
	return &Server{
		Cfg:             cfg,
		Port:            cfg.Port,
		Sites:           sites,
		agents: map[string]*agent.Agent{},
	}
}

// PublicBaseURL returns the address sites are reachable at.
func (s *Server) PublicBaseURL() string {
	return fmt.Sprintf("http://localhost:%d", s.Port)
}

func (s *Server) Start(ctx context.Context, mkAgent func(userID string, onImage func(string)) *agent.Agent) error {
	mux := http.NewServeMux()

	// sites
	mux.Handle("/sites/", http.StripPrefix("/sites/", s.sitesHandler()))
	mux.Handle("/images/", http.StripPrefix("/images/", s.imagesHandler()))
	mux.Handle("/videos/", http.StripPrefix("/videos/", s.videosHandler()))

	// PayPal webhook — PayPal notifies us when a payment completes
	mux.HandleFunc("/webhook/paypal", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		raw := make([]byte, r.ContentLength)
		r.Body.Read(raw)
		go s.handlePayPalWebhook(raw)
		w.WriteHeader(http.StatusOK)
	})

	// product images (static)
	mux.HandleFunc("/products/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/products/")
		if strings.Contains(name, "..") || strings.Contains(name, "/") {
			http.NotFound(w, r)
			return
		}
		fp := filepath.Join(s.Cfg.Workspace, "products", name)
		http.ServeFile(w, r, fp)
	})

	// messenger webhook
	mux.HandleFunc("/webhook/messenger", func(w http.ResponseWriter, r *http.Request) {
		s.handleMessengerWebhook(w, r, mkAgent)
	})

	// whatsapp webhook
	mux.HandleFunc("/webhook/whatsapp", func(w http.ResponseWriter, r *http.Request) {
		s.handleWhatsAppWebhook(w, r, mkAgent)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "🦊 payfox is alive")
	})

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.Port))
	if err != nil {
		return err
	}
	s.http = &http.Server{Handler: mux}
	go func() { <-ctx.Done(); _ = s.http.Close() }()
	fmt.Printf("  🦊 payfox serving on :%d (webhooks: /webhook/messenger, /webhook/whatsapp; sites: /sites/)\n", s.Port)
	return s.http.Serve(ln)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func firstLine(s string, n int) string {
	for i, r := range s {
		if r == '\n' {
			return truncateStr(s[:i], n)
		}
	}
	return truncateStr(s, n)
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func (s *Server) videosHandler() http.Handler {
	root := filepath.Join(s.Cfg.Workspace, "videos")
	return http.FileServer(http.Dir(root))
}

func (s *Server) imagesHandler() http.Handler {
	root := filepath.Join(s.Cfg.Workspace, "images")
	return http.FileServer(http.Dir(root))
}

func looksLikeBuildTask(text string) bool {
	t := strings.ToLower(text)
	words := []string{"bikin", "buat", "buatkan", "membuat", "gambar", "desain", "build", "create", "make", "draw", "design", "generate", "tulis", "render", "bikinin", "buatin"}
	for _, w := range words {
		if strings.Contains(t, w) {
			return true
		}
	}
	return false
}

func (s *Server) sitesHandler() http.Handler {
	root := filepath.Join(s.Cfg.Workspace, "sites")
	fs := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// never list parent dirs
		p := r.URL.Path
		if strings.Contains(p, "..") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		fs.ServeHTTP(w, r)
	})
}

func (s *Server) agentFor(userID string, mk func(string, func(string)) *agent.Agent) *agent.Agent {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.agents[userID]
	if !ok {
		a = mk(userID, func(url string) { s.sendMessengerImage(userID, url) })
		if s.Cfg.MemoryDir != "" {
			a.LoadHistory(filepath.Join(s.Cfg.MemoryDir, userID+".json"))
		}
		// status updates: send tool progress to user (like terminal output)
		a.OnToolProgress = func(toolName string) {
			msg := toolStatusText(toolName)
			if msg != "" {
				s.sendMessenger(userID, msg)
			}
		}
		s.agents[userID] = a
		if s.welcomed == nil {
			s.welcomed = map[string]bool{}
		}
		if !s.welcomed[userID] {
			s.welcomed[userID] = true
			go s.sendMessenger(userID, "Welcome to PayFox Store!\n\nType 'catalog' to browse products, or just tell me what you want to buy.\nSecure checkout via PayPal 💳")
		}
	}
	return a
}

// toolStatusText maps tool names to friendly progress messages.
// These are system progress indicators (like terminal output), not AI replies.
func toolStatusText(tool string) string {
	switch tool {
	case "write_file":
		return "📝 Writing code…"
	case "edit_file":
		return "✏️ Editing file…"
	case "publish_site":
		return "🚀 Publishing site…"
	case "katalog":
		return "Opening catalog…"
	case "tambah_produk":
		return "Adding product…"
	case "hapus_produk":
		return "Removing product…"
	case "bayar_paypal":
		return "Creating PayPal checkout…"
	case "paypal_checkout":
		return "Creating PayPal checkout…"
	case "paypal_invoice":
		return "Creating invoice…"
	case "gen_image":
		return "🎨 Generating image…"
	case "session_create":
		return "🔌 Starting browser…"
	case "page_open":
		return "👀 Opening website to verify…"
	case "page_screenshot":
		return "📸 Taking screenshot…"
	case "page_snapshot":
		return "📖 Reading page content…"
	case "send_photo":
		return "📤 Sending result to chat…"
	case "list_files":
		return "📁 Checking workspace…"
	case "read_file":
		return "📖 Reading file…"
	default:
		return "⚙ " + tool + "…"
	}
}

func (s *Server) buildUserMessage(ctx context.Context, ev messenger.Event) (*model.Message, error) {
	// pure text
	if ev.Text != "" && len(ev.Attachments) == 0 {
		fmt.Printf("→ [%s] %s\n", ev.SenderID[:min(8, len(ev.SenderID))], ev.Text)
		return &model.Message{Role: "user", Content: ev.Text}, nil
	}

	// voice note / audio → Muse Voice Transcribe
	if len(ev.Attachments) > 0 {
		att := ev.Attachments[0]
		switch att.Type {
		case "audio":
			fmt.Printf("→ [%s] 🎙 voice note (downloading %d bytes) url=%s\n", ev.SenderID[:min(8, len(ev.SenderID))], 0, truncateStr(att.PayloadURL, 60))
			if att.PayloadURL == "" {
				return nil, fmt.Errorf("voice note: empty payload URL")
			}
			data, err := messenger.DownloadAttachment(att.PayloadURL)
			if err != nil {
				fmt.Printf("⚠ download failed: %v\n", err)
				return nil, err
			}
			fmt.Printf("  📥 downloaded %d bytes\n", len(data))
			client := meta.New(s.Cfg.MetaAPIKey, s.Cfg.BaseURL)
			tctx, tcancel := context.WithTimeout(ctx, 90*time.Second)
			defer tcancel()
			transcript, err := client.TranscribeAudio(tctx, data, "voice-note.mp4")
			if err != nil {
				fmt.Printf("⚠ transcribe failed: %v\n", err)
				return nil, fmt.Errorf("transcribe: %w", err)
			}
			fmt.Printf("  📝 transcript: %s\n", firstLine(transcript, 80))
			fmt.Printf("→ [%s] 🎙 voice note: %s\n", ev.SenderID[:min(8, len(ev.SenderID))], firstLine(transcript, 80))
			label := "The user sent a VOICE NOTE that says: \"" + transcript + "\" (reply in the same language)"
			return &model.Message{Role: "user", Content: label}, nil

		case "image":
			fmt.Printf("→ [%s] 📷 photo (downloading…)\n", ev.SenderID[:min(8, len(ev.SenderID))])
			data, err := messenger.DownloadAttachment(att.PayloadURL)
			if err != nil {
				return nil, err
			}
			b64 := base64.StdEncoding.EncodeToString(data)
			parts := []model.ContentPart{
				{Type: "text", Text: "(user sent this photo" + captionNote(ev.Text) + " — read it and respond naturally)"},
				{Type: "image_url", ImageURL: &model.ImageURL{URL: "data:image/jpeg;base64," + b64}},
			}
			return &model.Message{Role: "user", Content: parts}, nil

		case "video":
			fmt.Printf("→ [%s] 🎥 video (downloading…)\n", ev.SenderID[:min(8, len(ev.SenderID))])
			// Download + save to workspace + serve from our tunnel
			data, err := messenger.DownloadAttachment(att.PayloadURL)
			if err != nil {
				return nil, err
			}
			// Save to workspace/videos/
			videoDir := filepath.Join(s.Cfg.Workspace, "videos")
			os.MkdirAll(videoDir, 0o755)
			videoName := fmt.Sprintf("video-%d.mp4", time.Now().UnixNano())
			videoPath := filepath.Join(videoDir, videoName)
			if err := os.WriteFile(videoPath, data, 0o644); err != nil {
				return nil, err
			}
			// Serve from our public URL
			publicURL := s.Cfg.PublicURL + "/videos/" + videoName
			// Also serve locally
			fmt.Printf("  📥 video saved: %d bytes → %s\n", len(data), publicURL)

			// Build multimodal message with video_url content part
			parts := []model.ContentPart{
				{Type: "text", Text: "(user sent this video" + captionNote(ev.Text) + " — watch it and respond naturally)"},
				{Type: "video_url", VideoURL: &model.VideoURL{URL: publicURL}},
			}
			return &model.Message{Role: "user", Content: parts}, nil
		}
	}

	return nil, nil
}

func contentToString(c any) string {
	switch v := c.(type) {
	case string:
		return v
	case []model.ContentPart:
		var parts []string
		for _, p := range v {
			if p.Text != "" {
				parts = append(parts, p.Text)
			}
		}
		return strings.Join(parts, " ")
	}
	return fmt.Sprintf("%v", c)
}

func captionNote(t string) string {
	if t != "" {
		return " with caption: \"" + t + "\""
	}
	return ""
}

func (s *Server) handleMessengerWebhook(w http.ResponseWriter, r *http.Request, mk func(string, func(string)) *agent.Agent) {
	// verification handshake
	if r.Method == http.MethodGet {
		hub := r.URL.Query()
		if hub.Get("hub.mode") == "subscribe" && hub.Get("hub.verify_token") == s.Cfg.Messenger.VerifyToken {
			fmt.Fprint(w, hub.Get("hub.challenge"))
			return
		}
		http.Error(w, "verify token mismatch", http.StatusForbidden)
		return
	}

	events, err := messenger.ParseWebhook(r.Body)
	if err != nil {
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}
	ctx0 := context.Background()
	for _, ev := range events {
		if ev.Text == "" && len(ev.Attachments) == 0 {
			continue
		}

		// build user message: text / voice-note / photo
		userMsg, err := s.buildUserMessage(ctx0, ev)
		if err != nil || userMsg == nil {
			continue
		}
		userMsgRef := userMsg
		senderID := ev.SenderID
	// Wire template sender (product cards) — lazily init sender then expose to tools
	if s.msg == nil {
		s.msg = messenger.NewSender(s.Cfg.Messenger.AccessToken)
	}
	_ = senderID // used below
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("⚠ PANIC recovered: %v\n", r)
				}
			}()
			// BUY BUTTON: user tapped "🛒 Buy Now" on a product card — instant PayPal checkout
			if strings.HasPrefix(ev.Text, "[BUY_BUTTON:") {
				prodID := strings.TrimSuffix(strings.TrimPrefix(ev.Text, "[BUY_BUTTON:"), "]")
				fmt.Printf("🛒 [BUY] user=%s product=%s\n", senderID, prodID)
				s.sendAction(senderID, "typing_on")
				if s.Store == nil {
					fmt.Printf("🛒 [BUY] ERROR: store is nil\n")
					s.sendMessenger(senderID, "Store is not ready yet. Please try again shortly.")
					return
				}
				order, link, err := s.Store.CheckoutTracked(prodID, senderID)
				if err != nil {
					fmt.Printf("🛒 [BUY] ERROR: %v\n", err)
					s.sendMessenger(senderID, fmt.Sprintf("❌ %v", err))
					return
				}
				fmt.Printf("🛒 [BUY] order=%s user=%s link=%s\n", order.OrderID, truncateStr(senderID, 10), truncateStr(link, 80))
				s.sendMessenger(senderID, fmt.Sprintf("🛒 %s\n💵 %s %s\n🔖 Order: %s\n\n✅ Tap this link to pay with PayPal:\n\n%s\n\nYou will get an automatic confirmation here once payment is complete 🦊",
					order.ProductName, order.Currency, order.Amount, order.OrderID[:8]+"…", link))
				return
			}

			// agent dibuat DI SINI (MCP attach kalau perlu) — webhook gak ke-block
			a := s.agentFor(senderID, mk)
			a.Tools.CurrentRecipient = senderID

			// typing indicator ON + auto-refresh (muse spark reasoning bisa lama)
			s.sendAction(senderID, "typing_on")
			ticker := time.NewTicker(12 * time.Second)
			done := make(chan struct{})
			defer close(done)
			go func() {
				for {
					select {
					case <-ticker.C:
						s.sendAction(senderID, "typing_on")
					case <-done:
						ticker.Stop()
						return
					}
				}
			}()

			// state per-message: kalau reply utama udah kekirim, drop ack
			var muAck sync.Mutex
			replySent := false

			// ── MUSE ACK (parallel): model yang mutusin + nulis ──
			go func() {
				ackCtx, ackCancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer ackCancel()
				ackClient := meta.New(s.Cfg.MetaAPIKey, s.Cfg.BaseURL)
				ackReq := []model.Message{
					{Role: "system", Content: "You are PayFox's intent router in a chat app. If the user's message asks to BUILD/CREATE/MAKE something (website, image, file, report), write ONE short friendly sentence (max 12 words) confirming you're working on it — IN ENGLISH, one emoji. If it's just a question/chat/greeting, reply with exactly: SKIP"},
					{Role: "user", Content: contentToString(userMsgRef.Content)},
				}
				resp, err := ackClient.Complete(ackCtx, meta.CompleteOptions{Messages: ackReq, MaxTokens: 3000})
				if err != nil {
					return
				}
				text := strings.TrimSpace(resp.Content)
				if text == "" || strings.Contains(strings.ToUpper(text), "SKIP") {
					return
				}
				muAck.Lock()
				if replySent {
					muAck.Unlock()
					return
				}
				muAck.Unlock()
				fmt.Printf("← ack (muse): %s\n", firstLine(text, 60))
				s.sendMessenger(senderID, text)
			}()

			// ── MAIN AGENT TASK ──
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
			defer cancel()
			reply, err := a.HandleMessage(ctx, *userMsgRef)
			if err != nil {
				reply = "ampun, error: " + err.Error()
			}
			muAck.Lock()
			replySent = true
			muAck.Unlock()
			fmt.Printf("← reply: %s\n", firstLine(reply, 90))
			s.sendMessenger(senderID, reply)
			if s.Cfg.MemoryDir != "" {
				a.PersistToFile(filepath.Join(s.Cfg.MemoryDir, senderID+".json"))
			}
		}()
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) sendAction(recipient, action string) {
	if s.msg == nil {
		s.msg = messenger.NewSender(s.Cfg.Messenger.AccessToken)
	}
	_ = s.msg.Action(recipient, action)
}

func (s *Server) sendMessengerImage(recipient, imageURL string) {
	if s.msg == nil {
		s.msg = messenger.NewSender(s.Cfg.Messenger.AccessToken)
	}
	if s.Cfg.Messenger.AccessToken == "" {
		return
	}
	if err := s.msg.SendImage(recipient, imageURL); err != nil {
		fmt.Printf("⚠ image send: %v\n", err)
	} else {
		fmt.Printf("🖼 image sent to chat: %s\n", imageURL)
	}
}

// SendTemplate sends a structured Messenger template (product cards) to a user.
func (s *Server) SendTemplate(recipient string, payload map[string]any) error {
	if s.msg == nil {
		s.msg = messenger.NewSender(s.Cfg.Messenger.AccessToken)
	}
	return s.msg.SendTemplate(recipient, payload)
}


// handlePayPalWebhook processes PayPal webhook events and notifies the
// Messenger user whose order was paid.
func (s *Server) handlePayPalWebhook(raw []byte) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("⚠ PAYPAL WEBHOOK PANIC: %v\n", r)
		}
	}()
	var ev struct {
		ID        string `json:"id"`
		EventType string `json:"event_type"`
		Resource  struct {
			ID   string `json:"id"`
			Status string `json:"status"`
			SupplementaryData struct {
				RelatedIDs struct {
					OrderID string `json:"order_id"`
				} `json:"related_ids"`
			} `json:"supplementary_data"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		fmt.Printf("⚠ PAYPAL WEBHOOK parse error: %v\n", err)
		return
	}

	// Find the order ID (different for different event types)
	orderID := ev.Resource.ID
	if ev.Resource.SupplementaryData.RelatedIDs.OrderID != "" {
		orderID = ev.Resource.SupplementaryData.RelatedIDs.OrderID
	}

	fmt.Printf("💳 [PAYPAL WEBHOOK] event=%s order=%s\n", ev.EventType, orderID)

	if s.Store == nil {
		fmt.Println("💳 [PAYPAL WEBHOOK] store nil — ignoring")
		return
	}
	order := s.Store.HandleWebhookEvent(ev.EventType, orderID)
	if order == nil {
		fmt.Printf("💳 [PAYPAL WEBHOOK] no tracked order for %s (event %s)\n", orderID, ev.EventType)
		return
	}

	// Notify the buyer in Messenger
	switch order.Status {
	case "APPROVED":
		s.sendMessenger(order.UserID, fmt.Sprintf("✅ Payment approved! Order %s — %s (%s %s). Processing now 🦊",
			order.OrderID[:8]+"…", order.ProductName, order.Currency, order.Amount))
	case "COMPLETED":
		// BEAUTIFUL RECEIPT CARD (Messenger Receipt Template)
		if s.msg == nil {
			s.msg = messenger.NewSender(s.Cfg.Messenger.AccessToken)
		}
		first, last := s.msg.GetUserName(order.UserID)
		if s.Store != nil {
			receipt := s.Store.ReceiptTemplate(order, first, last)
			if err := s.msg.SendTemplate(order.UserID, receipt); err != nil {
				// fallback to text if template fails
				s.sendMessenger(order.UserID, fmt.Sprintf("🎉 Payment SUCCESSFUL!\n\n📦 %s\n💵 %s %s\n🔖 Order: %s\n\nThank you for your purchase! 🦊☕",
					order.ProductName, order.Currency, order.Amount, order.OrderID[:8]+"…"))
			} else {
				s.sendMessenger(order.UserID, "Thank you for shopping with PayFox! 🦊 Your order is being processed.")
			}
		}
	}
	fmt.Printf("💳 [PAYPAL WEBHOOK] notified user=%s status=%s\n", order.UserID[:8], order.Status)
}


func (s *Server) sendMessenger(recipient, text string) {
	if s.msg == nil {
		s.msg = messenger.NewSender(s.Cfg.Messenger.AccessToken)
	}
	if s.msg == nil || s.Cfg.Messenger.AccessToken == "" {
		return
	}
	_ = s.msg.Send(recipient, text)
}

func (s *Server) handleWhatsAppWebhook(w http.ResponseWriter, r *http.Request, mk func(string, func(string)) *agent.Agent) {
	if r.Method == http.MethodGet {
		q := r.URL.Query()
		if q.Get("hub.verify_token") == s.Cfg.WhatsApp.VerifyToken {
			fmt.Fprint(w, q.Get("hub.challenge"))
			return
		}
		http.Error(w, "verify", http.StatusForbidden)
		return
	}
	// WhatsApp Cloud API events handled in D3
	w.WriteHeader(http.StatusOK)
}
