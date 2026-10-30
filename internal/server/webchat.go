package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/autokeren/payfox/internal/agent"
	"github.com/autokeren/payfox/internal/commerce"
	"github.com/autokeren/payfox/internal/config"
	"github.com/autokeren/payfox/internal/meta"
	"github.com/autokeren/payfox/internal/model"
	"github.com/autokeren/payfox/internal/sitefs"
	"github.com/autokeren/payfox/internal/tools"
)

// chatPage is the embedded HTML for the web chat interface.
const chatPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>PayFox 🦊 — AI Commerce Agent</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0a0a12; color: #e0e0e0; display: flex; justify-content: center; min-height: 100vh; }
  .container { max-width: 720px; width: 100%; padding: 20px; display: flex; flex-direction: column; height: 100vh; }
  header { text-align: center; padding: 20px 0 10px; }
  header h1 { font-size: 1.8em; color: #ff8a5f; }
  header p { color: #667788; font-size: 0.9em; margin-top: 5px; }
  .suggestions { padding: 10px 0; text-align: center; }
  .suggestions button { background: #1a1a2e; border: 1px solid #333; color: #c8f169; padding: 8px 16px; border-radius: 20px; margin: 4px; cursor: pointer; font-size: 0.85em; }
  .suggestions button:hover { background: #2a2a4e; }
  #chatbox { flex: 1; overflow-y: auto; padding: 15px; border-radius: 12px; background: #12121e; border: 1px solid #222; margin: 10px 0; }
  .msg { margin: 12px 0; padding: 12px 16px; border-radius: 16px; max-width: 85%; line-height: 1.5; font-size: 0.95em; white-space: pre-wrap; }
  .msg.user { background: #ff8a5f; color: #0a0a12; margin-left: auto; border-bottom-right-radius: 4px; }
  .msg.bot { background: #1e1e30; color: #e0e0e0; margin-right: auto; border-bottom-left-radius: 4px; }
  .msg.bot .tools { font-size: 0.75em; color: #c8f169; margin-bottom: 6px; font-family: monospace; }
  .typing { color: #667788; font-style: italic; padding: 8px 0; }
  .input-row { display: flex; gap: 10px; padding: 10px 0; }
  .input-row input { flex: 1; padding: 14px 18px; border-radius: 24px; border: 1px solid #333; background: #1a1a2e; color: #e0e0e0; font-size: 1em; outline: none; }
  .input-row input:focus { border-color: #ff8a5f; }
  .input-row button { padding: 14px 24px; border-radius: 24px; border: none; background: #ff8a5f; color: #0a0a12; font-weight: bold; cursor: pointer; font-size: 1em; }
  .input-row button:disabled { background: #444; cursor: not-allowed; }
  footer { text-align: center; padding: 10px; font-size: 0.75em; color: #444; }
</style>
</head>
<body>
<div class="container">
  <header>
    <h1>PayFox 🦊</h1>
    <p>AI Commerce Agent — Powered by Muse Spark + Qloo Taste AI + PayPal</p>
  </header>
  <div class="suggestions">
    <button onclick="suggest('I like jazz and French films. What coffee brands suit my taste?')">🎷 Jazz taste → coffee</button>
    <button onclick="suggest('I love indie rock and hiking. Recommend me some brands.')">🥾 Indie + hiking</button>
    <button onclick="suggest('Show me the catalog')">🛍️ Catalog</button>
    <button onclick="suggest('I enjoy sci-fi movies and electronic music. What brands would I like?')">🚀 Sci-fi + EDM</button>
  </div>
  <div id="chatbox">
    <div class="msg bot">Hi! I'm PayFox 🦊 — your AI shopping agent.<br><br>Tell me what you <b>like</b> (music, films, hobbies) and I'll recommend products matched to your taste using <b>Qloo's Taste AI</b>.<br><br>Or type <b>catalog</b> to browse our store and buy with PayPal.</div>
  </div>
  <div class="input-row">
    <input type="text" id="userInput" placeholder="Tell me what you like..." onkeypress="if(event.key==='Enter')send()">
    <button id="sendBtn" onclick="send()">Send</button>
  </div>
  <footer>Built for Qloo Agentic Hackathon + PayPal AI Hackathon · <a href="https://github.com/autokeren/payfox" style="color:#66aaff">github.com/autokeren/payfox</a></footer>
</div>
<script>
const chatbox = document.getElementById('chatbox');
const input = document.getElementById('userInput');
const btn = document.getElementById('sendBtn');

function suggest(text) { input.value = text; send(); }
function addMsg(text, cls) {
  const d = document.createElement('div');
  d.className = 'msg ' + cls;
  d.innerHTML = text;
  chatbox.appendChild(d);
  chatbox.scrollTop = chatbox.scrollHeight;
}
function showTyping() {
  const t = document.createElement('div');
  t.className = 'typing'; t.id = 'typing';
  t.textContent = 'PayFox is thinking…';
  chatbox.appendChild(t);
  chatbox.scrollTop = chatbox.scrollHeight;
}
function hideTyping() { const t = document.getElementById('typing'); if(t) t.remove(); }

async function send() {
  const text = input.value.trim();
  if (!text) return;
  input.value = '';
  addMsg(text.replace(/</g,'&lt;'), 'user');
  showTyping();
  btn.disabled = true;
  try {
    const res = await fetch('/chat/api', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: text })
    });
    const data = await res.json();
    hideTyping();
    let html = data.reply ? data.reply.replace(/\n/g, '<br>').replace(/</g,'&lt;').replace(/&lt;br&gt;/g,'<br>') : 'Sorry, I could not process that.';
    if (data.tools && data.tools.length > 0) {
      html = '<span class="tools">⚙️ ' + data.tools.join(' → ') + '</span><br>' + html;
    }
    addMsg(html, 'bot');
  } catch(e) {
    hideTyping();
    addMsg('Connection error. Please try again.', 'bot');
  }
  btn.disabled = false;
  input.focus();
}
input.focus();
</script>
</body>
</html>`

// handleChatPage serves the web chat interface.
func (s *Server) handleChatPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, chatPage)
}

// ChatRequest is the JSON body for the chat API.
type ChatRequest struct {
	Message string `json:"message"`
}

// ChatResponse is the JSON reply from the chat API.
type ChatResponse struct {
	Reply string   `json:"reply"`
	Tools []string `json:"tools,omitempty"`
}

// handleChatAPI processes a chat message and returns the agent's reply.
func (s *Server) handleChatAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Build a web-mode agent (no Messenger sender, tools logged instead)
	mk := func(userID string, onImage func(string)) *agent.Agent {
		a := buildWebAgent(s, s.Cfg, userID)
		return a
	}
	userID := "web-" + r.RemoteAddr
	a := s.agentFor(userID, mk)

	// Track tool calls for display
	var toolsUsed []string
	origProgress := a.OnToolProgress
	a.OnToolProgress = func(toolName string) {
		toolsUsed = append(toolsUsed, toolName)
		_ = origProgress
	}

	// Run the agent
	reply, err := a.Handle(r.Context(), req.Message)
	if err != nil {
		json.NewEncoder(w).Encode(ChatResponse{Reply: fmt.Sprintf("error: %v", err)})
		return
	}
	json.NewEncoder(w).Encode(ChatResponse{Reply: reply, Tools: toolsUsed})
}


// buildWebAgent creates a lightweight agent for the web chat (no Messenger).
func buildWebAgent(s *Server, cfg *config.Config, userID string) *agent.Agent {
	client := meta.New(cfg.MetaAPIKey, cfg.BaseURL)
	sites := sitefs.New(cfg.Workspace, cfg.PublicURL)
	reg := tools.New(cfg.Workspace, cfg.PublicURL, client, sites)

	if s.GlobalCatalog != nil {
		store := s.Store
		if store == nil {
			tracker := s.Tracker
			if tracker == nil {
				tracker, _ = commerce.NewOrderTracker(cfg.Workspace)
			}
			store = commerce.New(s.GlobalCatalog, nil, cfg.PublicURL, tracker)
		}
		reg.SetStore(store)
		reg.SetCatalog(s.GlobalCatalog)
	}
	if s.Store != nil && s.Store.PayPal != nil {
		reg.SetPayPal(s.Store.PayPal)
	}
	if s.Qloo != nil {
		reg.SetQloo(s.Qloo)
	}
	return agent.New(client, reg)
}

// buildUserMessageFromText creates a model.Message from plain chat text.
func (s *Server) buildUserMessageFromText(text string) *model.Message {
	return &model.Message{
		Role:    "user",
		Content: text,
	}
}
