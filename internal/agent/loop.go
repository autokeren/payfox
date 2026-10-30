package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"fmt"
	"strings"

	"github.com/autokeren/payfox/internal/meta"
	"github.com/autokeren/payfox/internal/model"
	"github.com/autokeren/payfox/internal/tools"
)

const systemPrompt = `You are PayFox 🦊 — an AI commerce agent living in Facebook Messenger.
You run a STORE inside chat: show products, create PayPal payment links, manage the catalog.
You also build websites/images when asked.

LANGUAGE RULE (CRITICAL): ALWAYS respond in ENGLISH — no exceptions.
Even if the user writes in Bahasa Indonesia, Spanish, or any other
language, your reply MUST be in English. This is an English-language
product for an international audience.

TOOL ROUTING — READ CAREFULLY:

User wants to SEE products:
  "catalog" / "show catalog" / "browse" / "what do you have" / "katalog"
  → CALL: katalog (NO ARGUMENTS)
  → This sends beautiful PRODUCT CARDS with Buy buttons.

Store owner wants to ADD a product:
  "add product" / "list an item" / "I want to sell" / "tambah produk"
  → CALL: tambah_produk(name, price, currency)
  → NEVER call this when the user just wants to browse!

Customer wants to BUY / PAY:
  "buy this" / "checkout" / "I want the coffee" / "bayar" / product_id known
  → CALL: bayar_paypal(product_id="prod_X")
  → Or for custom orders: bayar_paypal(item_name="...", amount="...", currency="USD")

FORMAL INVOICE (B2B, services, freelancers):
  "create an invoice" / "invoice my client $500"
  → CALL: paypal_invoice(item_name, amount, currency)

CRITICAL RULES:
1. "catalog" / "show products" = katalog tool. NEVER tambah_produk.
2. "add product" / "want to sell" = tambah_produk tool. NEVER katalog.
3. "buy" / "pay" / "checkout" = bayar_paypal tool.
4. Keep replies SHORT (1-2 sentences). This is a chat, not a document.
5. Voice notes arrive as transcripts — treat them like normal messages.
6. Multi-step tasks: just do them all. Never say "I can't".
7. After creating a checkout link, tell the customer they will get an
   automatic confirmation message in chat after payment completes.`

const MaxSteps = 8

type Agent struct {
	Client          *meta.Client
	Tools           *tools.Registry
	History         []model.Message
	OnFirstToolCall  func()      // fired once when Muse decides to build
	OnToolProgress  func(string) // fired on EVERY tool call (status update)
	firstToolFired   bool
	MemoryDir       string     // where history persists
	ContextLimit     int        // model context window (tokens)
	CompactAt        float64    // fraction of ContextLimit that triggers compaction
}

const DefaultContextLimit = 1_000_000 // muse-spark-1.3: 1M

func New(client *meta.Client, reg *tools.Registry) *Agent {
	return &Agent{Client: client, Tools: reg}
}

// Handle processes one text message.
func (a *Agent) Handle(ctx context.Context, userText string) (string, error) {
	return a.HandleMessage(ctx, model.Message{Role: "user", Content: userText})
}

// HandleMessage processes one pre-built user message (text OR multimodal).
// Multi-step: executes tool calls until the model answers with plain content.
func (a *Agent) HandleMessage(ctx context.Context, userMsg model.Message) (string, error) {
	a.History = append(a.History, userMsg)
	if len(a.History) == 1 {
		a.History = append([]model.Message{{Role: "system", Content: systemPrompt}}, a.History...)
	}

	toolDefs := a.Tools.Definitions()
	fmt.Printf("    🔧 sending %d tools to Muse: ", len(toolDefs))
	for _, td := range toolDefs {
		fmt.Printf("%s ", td.Function.Name)
	}
	fmt.Println()

	for step := 0; step < MaxSteps; step++ {
		resp, err := a.Client.Complete(ctx, meta.CompleteOptions{
			Messages:  a.History,
			Tools:     toolDefs,
			MaxTokens: 16000,
		})
		if err != nil {
			return "", fmt.Errorf("muse-spark: %w", err)
		}

		// opencode-style memory: persist every turn + compact near the limit
		a.persist()
		limit := a.ContextLimit
		if limit <= 0 {
			limit = DefaultContextLimit
		}
		if a.CompactAt <= 0 {
			a.CompactAt = 0.85
		}
		if resp.Usage.PromptTokens > int(float64(limit)*a.CompactAt) {
			fmt.Printf("    🗜 compaction: %d tokens > %.0f%% of context — summarizing…\n", resp.Usage.PromptTokens, a.CompactAt*100)
			a.compact(ctx)
		}

		msg := model.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		a.History = append(a.History, msg)

		if len(resp.ToolCalls) == 0 {
			// plain answer — done
			if strings.TrimSpace(resp.Content) == "" {
				return "(muse berpikir lama tapi belum menjawab — coba lagi)", nil
			}
			return resp.Content, nil
		}

		// model-triggered ack: Muse mutusin buat ngebuild → kabarin user
		if len(resp.ToolCalls) > 0 && !a.firstToolFired {
			a.firstToolFired = true
			if a.OnFirstToolCall != nil {
				a.OnFirstToolCall()
			}
		}

		// execute each tool call, append results
		for _, call := range resp.ToolCalls {
			fmt.Printf("    ⚙ %s(%s)\n", call.Function.Name, shortArgs(call.Function.Arguments))
			// status update to user (every tool call after the first)
			if a.firstToolFired && a.OnToolProgress != nil {
				a.OnToolProgress(call.Function.Name)
			}
			result := a.Tools.Execute(ctx, call)
			a.History = append(a.History, model.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: call.ID,
			})
		}
	}
	return "Maaf, langkahnya kepanjangan 😅 — coba task yang lebih spesifik?", nil
}

// Reset clears conversation memory (new session per user is default).
func (a *Agent) Reset() {
	a.History = nil
	a.firstToolFired = false
}

// memoryPath returns the JSON file this conversation persists to.
func (a *Agent) memoryPath(userID string) string {
	if a.MemoryDir == "" {
		return ""
	}
	return filepath.Join(a.MemoryDir, userID+".json")
}

// PersistToFile saves history (used with the server's per-user wiring).
func (a *Agent) PersistToFile(path string) {
	if path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	raw, _ := json.Marshal(a.History)
	_ = os.WriteFile(path, raw, 0o600)
}

func (a *Agent) persist() {
	if a.memoryPath("session") != "" {
		a.PersistToFile(filepath.Join(a.MemoryDir, "_last.json"))
	}
}

// LoadHistory replaces in-RAM history with the persisted one.
func (a *Agent) LoadHistory(path string) {
	if path == "" {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var h []model.Message
	if json.Unmarshal(raw, &h) == nil && len(h) > 0 {
		a.History = h
	}
}

// compact summarizes older turns and keeps the recent tail verbatim,
// mirroring opencode's context compaction at the threshold.
func (a *Agent) compact(ctx context.Context) {
	const keep = 6 // keep the last 6 messages verbatim
	if len(a.History) <= keep+2 {
		return
	}
	old := a.History[1 : len(a.History)-keep] // everything after system, before tail
	tail := a.History[len(a.History)-keep:]

	var sb strings.Builder
	for _, m := range old {
		role := m.Role
		text := contentToString(m.Content)
		if len(text) > 500 {
			text = text[:500] + "…"
		}
		if len(m.ToolCalls) > 0 {
			role = "assistant(tool call)"
			text = m.ToolCalls[0].Function.Name
		}
		sb.WriteString(role + ": " + text + "\n")
	}

	summaryReq := []model.Message{
		{Role: "system", Content: "Summarize this conversation history for an agent named PayFox. Keep: what the user asked for, what was built (site names, image names, URLs), and any preferences. Max 200 words. Reply with ONLY the summary."},
		{Role: "user", Content: sb.String()},
	}
	resp, err := a.Client.Complete(ctx, meta.CompleteOptions{Messages: summaryReq, MaxTokens: 3000})
	if err != nil {
		// fallback: drop old messages hard
		a.History = append([]model.Message{a.History[0]}, tail...)
		return
	}
	summary := resp.Content
	a.History = append(
		[]model.Message{a.History[0], {Role: "user", Content: "[memory] Sebelumnya: " + summary}},
		tail...,
	)
}

var _ = json.Marshal

func contentToString(c any) string {
	switch v := c.(type) {
	case string:
		return v
	case []model.ContentPart:
		var parts []string
		for _, p := range v {
			if p.Text != "" {
				parts = append(parts, p.Text)
			} else if p.ImageURL != nil {
				parts = append(parts, "[image]")
			}
		}
		return strings.Join(parts, " ")
	}
	return fmt.Sprintf("%v", c)
}

func shortArgs(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 60 {
		return s[:60] + "…"
	}
	return s
}
