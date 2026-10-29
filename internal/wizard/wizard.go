package wizard

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/autokeren/payfox/internal/config"
	"github.com/autokeren/payfox/internal/meta"
)

const banner = `
  ╭──────────────────────────────────────────╮
  │   🦌  K A N C I L                        │
  │   the clever agent in your Messenger     │
  │   brain: Meta Muse Spark 1.3             │
  ╰──────────────────────────────────────────╯
`

func Run(reader *bufio.Reader) (*config.Config, error) {
	fmt.Print(banner)
	fmt.Println("  Let's get you live in 3 steps.\n")

	cfg, _ := config.Load()
	if cfg == nil {
		home, _ := os.UserHomeDir()
		cfg = &config.Config{Workspace: filepath.Join(home, "kancil-builds")}
	}

	// ── Step 1: Meta Model API key ─────────────────────────────
	if cfg.MetaAPIKey != "" {
		fmt.Printf("  ✓ Meta API key: %s… (saved)\n", short(cfg.MetaAPIKey))
		fmt.Print("    press Enter to keep, or paste a new key: ")
		if in := readLine(reader); strings.TrimSpace(in) != "" {
			cfg.MetaAPIKey = strings.TrimSpace(in)
		}
	} else {
		fmt.Print("  1/3  Meta Model API key (get it: your Meta Model API dashboard\n        → API keys): ")
		cfg.MetaAPIKey = readSecret(reader)
	}
	fmt.Print("    verifying key against api.meta.ai… ")
	client := meta.New(cfg.MetaAPIKey, cfg.BaseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := client.Verify(ctx); err != nil {
		cancel()
		fmt.Println("✗")
		return nil, fmt.Errorf("key invalid: %v", err)
	}
	cancel()
	fmt.Println("✓ muse-spark-1.3 ready")

	// ── Step 2: channels ───────────────────────────────────────
	fmt.Println()
	fmt.Println("  2/3  Channels — you can connect both or skip for now.")
	cfg.Messenger.AccessToken = askOrKeep(reader, "  Messenger Page Access Token", cfg.Messenger.AccessToken)
	cfg.Messenger.VerifyToken = askOrKeep(reader, "  Messenger Verify Token", cfg.Messenger.VerifyToken)
	cfg.Messenger.AppSecret = askOrKeep(reader, "  Messenger App Secret", cfg.Messenger.AppSecret)
	cfg.Messenger.PageID = askOrKeep(reader, "  Messenger Page ID", cfg.Messenger.PageID)
	fmt.Println()
	cfg.WhatsApp.AccessToken = askOrKeep(reader, "  WhatsApp Access Token", cfg.WhatsApp.AccessToken)
	cfg.WhatsApp.PhoneID = askOrKeep(reader, "  WhatsApp Phone Number ID", cfg.WhatsApp.PhoneID)
	cfg.WhatsApp.VerifyToken = askOrKeep(reader, "  WhatsApp Verify Token", cfg.WhatsApp.VerifyToken)

	// ── Step 3: workspace ──────────────────────────────────────
	fmt.Println()
	fmt.Printf("  3/3  Workspace folder (agent builds things here)\n        [%s]: ", cfg.Workspace)
	if in := readLine(reader); strings.TrimSpace(in) != "" {
		cfg.Workspace = strings.TrimSpace(in)
	}
	if err := os.MkdirAll(cfg.Workspace, 0o755); err != nil {
		return nil, err
	}

	if err := cfg.Save(); err != nil {
		return nil, err
	}
	fmt.Println()
	fmt.Println("  ✅ Saved to " + config.Path())
	return cfg, nil
}

func askOrKeep(r *bufio.Reader, label, current string) string {
	if current != "" {
		fmt.Printf("%s: ✓ %s… (Enter = keep, new = replace)\n           ", label, short(current))
		if in := readLine(r); strings.TrimSpace(in) != "" {
			return strings.TrimSpace(in)
		}
		return current
	}
	fmt.Printf("%s (Enter = skip): ", label)
	return strings.TrimSpace(readLine(r))
}

func readLine(r *bufio.Reader) string {
	line, _ := r.ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}

func readSecret(r *bufio.Reader) string {
	line := readLine(r)
	return strings.TrimSpace(line)
}

func short(s string) string {
	if len(s) <= 10 {
		return s
	}
	return s[:6] + "…" + s[len(s)-4:]
}
