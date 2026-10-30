package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/autokeren/payfox/internal/agent"
	"github.com/autokeren/payfox/internal/catalog"
	"github.com/autokeren/payfox/internal/commerce"
	"github.com/autokeren/payfox/internal/config"
	"github.com/autokeren/payfox/internal/mcp"
	"github.com/autokeren/payfox/internal/meta"
	"github.com/autokeren/payfox/internal/paypal"
	"github.com/autokeren/payfox/internal/qloo"
	"github.com/autokeren/payfox/internal/server"
	"github.com/autokeren/payfox/internal/sitefs"
	"github.com/autokeren/payfox/internal/tools"
	"github.com/autokeren/payfox/internal/wizard"
)

const version = "0.1.0"

var paypalClient *paypal.Client
var qlooClient *qloo.Client
var sharedCatalog *catalog.Catalog // SINGLETON — one catalog for everything
var sharedTracker *commerce.OrderTracker // SINGLETON — order tracking for webhooks
var templateSenderFn func(recipient string, payload map[string]any) error

func main() {
	args := os.Args[1:]
	switch {
	case len(args) == 0:
		runDefault()
	case args[0] == "chat":
		runChat()
	case args[0] == "serve":
		runServe()
	case args[0] == "wizard":
		runWizard()
	case args[0] == "--version":
		fmt.Println("kancil", version)
	default:
		fmt.Println("kancil — the clever agent in your Messenger 🦌")
		fmt.Println("  kancil        setup wizard + serve")
		fmt.Println("  kancil chat   chat locally in terminal")
		fmt.Println("  kancil serve  run webhook server only")
	}
}

func ensureConfig() (*config.Config, bool) {
	cfg, _ := config.Load()
	if cfg != nil && cfg.Validate() == nil {
		return cfg, true
	}
	fmt.Println("  no config found — starting wizard\n")
	newCfg, err := wizard.Run(bufio.NewReader(os.Stdin))
	if err != nil {
		fmt.Println("  ✗", err)
		os.Exit(1)
	}
	return newCfg, true
}

func runDefault() {
	cfg, _ := ensureConfig()
	_ = cfg
	runServe()
}

func runWizard() {
	if _, err := wizard.Run(bufio.NewReader(os.Stdin)); err != nil {
		fmt.Println("  ✗", err)
		os.Exit(1)
	}
}

func buildAgent(cfg *config.Config, _ string, onImage func(string)) *agent.Agent {
	return buildAgentWithMCP(cfg, onImage, nil)
}

func buildAgentWithMCP(cfg *config.Config, onImage func(string), attachMCP func(*tools.Registry)) *agent.Agent {
	client := meta.New(cfg.MetaAPIKey, cfg.BaseURL)
	sites := sitefs.New(cfg.Workspace, cfg.PublicURL)
	reg := tools.New(cfg.Workspace, cfg.PublicURL, client, sites)

	// Commerce: use the SHARED singleton catalog (same instance as server)
	if sharedCatalog != nil {
		tracker := sharedTracker
		if tracker == nil {
			tracker, _ = commerce.NewOrderTracker(cfg.Workspace)
		}
		store := commerce.New(sharedCatalog, paypalClient, cfg.PublicURL, tracker)
		reg.SetStore(store)
		reg.SetCatalog(sharedCatalog)
	}
	reg.OnImageSaved = onImage
	if attachMCP != nil {
		attachMCP(reg)
	}
	if paypalClient != nil {
		reg.SetPayPal(paypalClient)
	}
	if qlooClient != nil {
		reg.SetQloo(qlooClient)
	}
	if templateSenderFn != nil {
		reg.SetTemplateSender(templateSenderFn)
	}
	return agent.New(client, reg)
}

func runServe() {
	cfg, _ := ensureConfig()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := server.New(cfg, sitefs.New(cfg.Workspace, cfg.PublicURL))

	// Wire the Messenger template sender (for product cards) into the global registry
	templateSenderFn = func(recipient string, payload map[string]any) error {
		return srv.SendTemplate(recipient, payload)
	}

	// SINGLETON catalog — load ONCE, share with agents + server
	{
		var catErr error
		sharedCatalog, catErr = catalog.Load(cfg.Workspace)
		if catErr != nil {
			fmt.Println("⚠️  catalog load:", catErr)
		}
	}
	sharedTracker, _ = commerce.NewOrderTracker(cfg.Workspace)
	if sharedCatalog != nil {
		store := commerce.New(sharedCatalog, paypalClient, cfg.PublicURL, sharedTracker)
		srv.Store = store
		srv.Tracker = sharedTracker
	}

	// PayPal: wire from env (sandbox by default)
	if cid := os.Getenv("PAYPAL_CLIENT_ID"); cid != "" {
		secret := os.Getenv("PAYPAL_CLIENT_SECRET")
		sandbox := os.Getenv("PAYPAL_LIVE") != "1"
		pc := paypal.New(cid, secret, sandbox)
		if err := pc.Verify(); err != nil {
			fmt.Println("⚠️  PayPal credentials invalid:", err)
		} else {
			mode := "LIVE"
			if sandbox {
				mode = "SANDBOX"
			}
			fmt.Println("🦊 PayPal " + mode + " connected")
		}
		attachPayPal := func(reg *tools.Registry) { reg.SetPayPal(pc) }
		_ = attachPayPal
		paypalClient = pc
	}

	// Qloo Taste AI — wire from env
	if qkey := os.Getenv("QLOO_API_KEY"); qkey != "" {
		qlooClient = qloo.New(qkey)
		fmt.Println("🧠 Qloo Taste AI connected")
	}

	// CRITICAL FIX: Store was created BEFORE PayPal was ready — inject PayPal client now
	if paypalClient != nil && srv != nil && srv.Store != nil {
		srv.Store.PayPal = paypalClient
	}

	// MCP servers: connect SEKALI di startup, share antar agent
	var mcpMutex sync.Mutex

	attachMCP := func(reg *tools.Registry) {
		mcpMutex.Lock()
		defer mcpMutex.Unlock()
		for _, mc := range cfg.MCPServers {
			if reg.MCPToolCount() > 0 {
				continue // registry already has MCP (shared user session)
			}
			srv := mcp.NewServer(mc.Name, mc.Command, mc.Env)
			if err := reg.AttachMCP(ctx, srv); err != nil {
				fmt.Printf("  ⚠ MCP %s: %v\n", mc.Name, err)
			}
		}
	}

	if err := srv.Start(ctx, func(userID string, onImage func(string)) *agent.Agent {
		return buildAgentWithMCP(cfg, onImage, attachMCP)
	}); err != nil {
		fmt.Println("  ✗ server:", err)
		os.Exit(1)
	}
}

func runChat() {
	cfg, _ := ensureConfig()
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()
	a := buildAgentWithMCP(cfg, func(url string) {
		fmt.Printf("  🖼 %s\n", url)
	}, func(reg *tools.Registry) {
		for _, mc := range cfg.MCPServers {
			srv := mcp.NewServer(mc.Name, mc.Command, mc.Env)
			if err := reg.AttachMCP(ctx, srv); err != nil {
				fmt.Printf("  ⚠ MCP %s: %v\n", mc.Name, err)
			}
		}
	})
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("  🦌 kancil chat — muse-spark-1.3 (Ctrl+C to exit)\n")
	for {
		fmt.Print("  you ▸ ")
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			return
		}
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		reply, err := a.Handle(ctx, text)
		cancel()
		if err != nil {
			fmt.Println("  ✗", err)
			continue
		}
		fmt.Printf("  kancil ▸ %s\n\n", reply)
	}
}
