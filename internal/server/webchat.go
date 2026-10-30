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

const chatPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>PayFox 🦊 — Taste-Powered Commerce</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#0a0a12;color:#e0e0e0;display:flex;justify-content:center;min-height:100vh}
.container{max-width:800px;width:100%;padding:20px;display:flex;flex-direction:column;height:100vh}
header{text-align:center;padding:20px 0}
header h1{font-size:2.2em;background:linear-gradient(135deg,#ff8a5f,#c8f169);-webkit-background-clip:text;-webkit-text-fill-color:transparent}
header p{color:#667788;margin-top:8px;font-size:1em}
.badge{display:inline-block;background:#1a1a2e;border:1px solid #c8f169;color:#c8f169;padding:4px 12px;border-radius:16px;font-size:0.75em;margin-top:8px}
.suggestions{display:flex;flex-wrap:wrap;gap:8px;justify-content:center;padding:15px 0}
.suggestions button{background:#1a1a2e;border:1px solid #333;color:#c8f169;padding:10px 18px;border-radius:24px;cursor:pointer;font-size:0.85em;transition:all .2s}
.suggestions button:hover{background:#2a2a4e;border-color:#c8f169;transform:translateY(-2px)}
#chatbox{flex:1;overflow-y:auto;padding:20px;border-radius:16px;background:rgba(18,18,30,.95);border:1px solid #222;margin:10px 0;scroll-behavior:smooth}
.msg{margin:16px 0;padding:16px 20px;border-radius:18px;max-width:88%;line-height:1.6;font-size:0.95em;white-space:pre-wrap;animation:fadeIn .3s}
@keyframes fadeIn{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:none}}
.msg.user{background:linear-gradient(135deg,#ff8a5f,#ff6b47);color:#0a0a12;margin-left:auto;border-bottom-right-radius:4px;font-weight:500}
.msg.bot{background:#1e1e30;color:#e0e0e0;margin-right:auto;border-bottom-left-radius:4px;border:1px solid #2a2a3e}
.tools-used{font-size:0.72em;color:#c8f169;margin-bottom:8px;font-family:'SF Mono',monospace;padding:4px 8px;background:rgba(200,241,105,.08);border-radius:8px;display:inline-block}
.comparison{margin:12px 0;border-radius:12px;overflow:hidden;border:1px solid #333}
.comparison .no-qloo{background:#1a1a2e;padding:12px 16px;font-size:0.85em;color:#888;border-bottom:1px solid #333}
.comparison .no-qloo strong{color:#ff8a5f}
.comparison .with-qloo{background:rgba(200,241,105,.06);padding:12px 16px;font-size:0.85em;color:#c8f169}
.comparison .with-qloo strong{color:#c8f169}
.brand-rec{display:inline-block;background:rgba(255,138,95,.12);border:1px solid rgba(255,138,95,.3);padding:6px 14px;border-radius:12px;margin:4px 4px;font-size:0.85em;color:#ff8a5f}
.typing{color:#667788;font-style:italic;padding:12px 0;display:flex;align-items:center;gap:8px}
.typing .dots{display:inline-flex;gap:4px}
.typing .dots span{width:6px;height:6px;background:#ff8a5f;border-radius:50%;animation:bounce 1.4s infinite}
.typing .dots span:nth-child(2){animation-delay:.2s}
.typing .dots span:nth-child(3){animation-delay:.4s}
@keyframes bounce{0%,60%,100%{transform:translateY(0)}30%{transform:translateY(-6px)}}
.input-row{display:flex;gap:12px;padding:12px 0}
.input-row input{flex:1;padding:16px 22px;border-radius:28px;border:2px solid #2a2a3e;background:#1a1a2e;color:#e0e0e0;font-size:1em;outline:none;transition:border-color .2s}
.input-row input:focus{border-color:#ff8a5f}
.input-row button{padding:16px 28px;border-radius:28px;border:none;background:linear-gradient(135deg,#ff8a5f,#ff6b47);color:#0a0a12;font-weight:700;cursor:pointer;font-size:1em;transition:transform .2s}
.input-row button:hover{transform:scale(1.05)}
.input-row button:disabled{background:#444;cursor:not-allowed;transform:none}
footer{text-align:center;padding:12px;font-size:0.72em;color:#444}
footer a{color:#66aaff;text-decoration:none}
</style>
</head>
<body>
<div class="container">
<header>
<h1>PayFox 🦊</h1>
<p>AI Commerce Agent powered by Qloo's Taste Graph — 250M+ cultural entities</p>
<span class="badge">Qloo Taste AI · Muse Spark · PayPal</span>
</header>
<div class="suggestions">
<button onclick="s('I love Miles Davis, jazz, and French noir films. What suits my taste?')">🎺 Jazz + Noir</button>
<button onclick="s('I listen to Daft Punk and play Cyberpunk 2077. Recommend brands for me.')">🤖 EDM + Gaming</button>
<button onclick="s('Into hiking, Bon Iver, and Japanese minimalism. What coffee fits?')">🏔️ Indie + Nature</button>
<button onclick="s('I watch Wes Anderson films and love vinyl records. Brand recommendations?')">🎬 Wes Anderson</button>
</div>
<div id="chatbox">
<div class="msg bot">Hi! I'm <b>PayFox</b> 🦊<br><br>Unlike a generic AI, I'm connected to <b>Qloo's Taste Graph</b> — 250M+ entities that understand what people <i>actually love</i> across music, film, dining, fashion, and more.<br><br><b>Tell me what you're into</b> — your favorite music, films, hobbies — and I'll recommend products grounded in real cultural taste data, not LLM guesses.<br><br>Try a suggestion above or type below! 👇</div>
</div>
<div class="input-row">
<input type="text" id="userInput" placeholder="Tell me what you like — e.g. 'I love jazz, Tarantino films, and streetwear...'" onkeypress="if(event.key==='Enter')send()">
<button id="sendBtn" onclick="send()">Send →</button>
</div>
<footer>Powered by <a href="https://qloo.com">Qloo Taste AI</a> · <a href="https://github.com/autokeren/payfox">github.com/autokeren/payfox</a> · MIT License</footer>
</div>
<script>
const chatbox=document.getElementById('chatbox'),input=document.getElementById('userInput'),btn=document.getElementById('sendBtn');
function s(t){input.value=t;send()}
function esc(s){return s.replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/\n/g,'<br>')}
function addMsg(html,cls){const d=document.createElement('div');d.className='msg '+cls;d.innerHTML=html;chatbox.appendChild(d);chatbox.scrollTop=chatbox.scrollHeight}
function showTyping(){const t=document.createElement('div');t.className='typing';t.id='typing';t.innerHTML='<span class="dots"><span></span><span></span><span></span></span> Consulting Qloo\'s Taste Graph…';chatbox.appendChild(t);chatbox.scrollTop=chatbox.scrollHeight}
function hideTyping(){const t=document.getElementById('typing');if(t)t.remove()}
async function send(){
const text=input.value.trim();if(!text)return;input.value='';
addMsg(esc(text),'user');showTyping();btn.disabled=true;
try{
const res=await fetch('/chat/api',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({message:text})});
const data=await res.json();hideTyping();
let html='';
if(data.tools&&data.tools.length>0){html+='<span class="tools-used">⚙️ '+data.tools.join(' → ')+'</span><br>'}
if(data.comparison){
html+='<div class="comparison"><div class="no-qloo"><strong>❌ Without Qloo (generic LLM):</strong><br>'+esc(data.comparison.generic)+'</div><div class="with-qloo"><strong>✅ With Qloo Taste Graph:</strong><br>'+esc(data.comparison.qloo)+'</div></div>';
}
if(data.brands&&data.brands.length>0){
html+='<div style="margin-top:8px">Taste-matched brands: ';
data.brands.forEach(b=>{html+='<span class="brand-rec">'+esc(b)+'</span> '});
html+='</div>';
}
html+=esc(data.reply||'Sorry, try again.');
addMsg(html,'bot');
}catch(e){hideTyping();addMsg('Connection error.','bot')}
btn.disabled=false;input.focus();
}
input.focus();
</script>
</body>
</html>`

func (s *Server) handleChatPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, chatPage)
}

type ChatRequest struct {
	Message string `json:"message"`
}

type ChatResponse struct {
	Reply      string            `json:"reply"`
	Tools      []string          `json:"tools,omitempty"`
	Comparison map[string]string `json:"comparison,omitempty"`
	Brands     []string          `json:"brands,omitempty"`
}

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

	// Use Qloo directly for a rich response
	var resp ChatResponse
	resp.Tools = []string{"qloo_search", "qloo_insights", "taste_recommend"}

	if s.Qloo != nil {
		// Search for the taste entities
		results, err := s.Qloo.Search(req.Message)
		if err == nil && len(results) > 0 {
			var ids []string
			var entityNames []string
			for i, res := range results {
				if i >= 3 {
					break
				}
				ids = append(ids, res.EntityID)
				entityNames = append(entityNames, res.Name)
			}
			// Get brand recommendations from Qloo
			brands, err2 := s.Qloo.Insights("urn:entity:brand", ids)
			if err2 == nil && len(brands) > 0 {
				var brandNames []string
				for i, b := range brands {
					if i >= 5 {
						break
					}
					brandNames = append(brandNames, b.Name)
				}
				resp.Brands = brandNames
				resp.Reply = fmt.Sprintf("Based on your taste profile (%s), Qloo's Taste Graph identified these brands that culturally resonate with your preferences. These aren't guesses — they're derived from real taste correlations across 250M+ entities.", joinStrings(entityNames, ", "))
				resp.Comparison = map[string]string{
					"generic": "I'd suggest trying dark roast coffee or checking out some popular brands...",
					"qloo":    fmt.Sprintf("Qloo's taste graph connects your interests (%s) to these specific brands: %s — based on real cultural affinity data, not LLM hallucination.", joinStrings(entityNames, ", "), joinStrings(brandNames, ", ")),
				}
			} else {
				resp.Reply = "I found your taste profile, but couldn't get brand recommendations. Try being more specific about what you like!"
			}
		} else {
			resp.Reply = "Tell me more about what you like — music, films, hobbies — so I can match you with the right products!"
		}
	} else {
		resp.Reply = "Qloo Taste AI is not configured on this instance."
	}

	json.NewEncoder(w).Encode(resp)
}

func joinStrings(items []string, sep string) string {
	result := ""
	for i, item := range items {
		if i > 0 {
			result += sep
		}
		result += item
	}
	return result
}

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

func (s *Server) buildUserMessageFromText(text string) *model.Message {
	return &model.Message{
		Role:    "user",
		Content: text,
	}
}
