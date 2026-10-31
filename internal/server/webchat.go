package server

import (
	"encoding/json"
	"strings"
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
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#0a0a12;color:#e0e0e0;display:flex;min-height:100vh}
.container{flex:1;display:flex;flex-direction:column;height:100vh;padding:12px 24px}
header{text-align:center;padding:14px 0 6px}
header h1{font-size:1.9em;background:linear-gradient(135deg,#ff8a5f,#c8f169);-webkit-background-clip:text;-webkit-text-fill-color:transparent;display:inline}
header p{color:#667788;margin-top:4px;font-size:0.9em}
.badge{display:inline-block;background:rgba(200,241,105,.08);border:1px solid rgba(200,241,105,.2);color:#c8f169;padding:3px 12px;border-radius:14px;font-size:0.72em;margin-left:10px;vertical-align:middle}
.suggestions{display:flex;flex-wrap:wrap;gap:8px;justify-content:center;padding:10px 0}
.suggestions button{background:rgba(26,26,46,.8);border:1px solid #2a2a3e;color:#c8f169;padding:9px 16px;border-radius:20px;cursor:pointer;font-size:0.8em;transition:all .2s}
.suggestions button:hover{background:#2a2a4e;border-color:#c8f169;transform:translateY(-2px)}
#chatbox{flex:1;overflow-y:auto;padding:16px 8px;border-radius:14px;background:rgba(15,15,25,.6);border:1px solid #1a1a2e;margin:8px 0;scroll-behavior:smooth}
#chatbox::-webkit-scrollbar{width:5px}
#chatbox::-webkit-scrollbar-track{background:transparent}
#chatbox::-webkit-scrollbar-thumb{background:#2a2a3e;border-radius:3px}
.msg{margin:14px 8px;padding:14px 18px;border-radius:16px;max-width:78%;line-height:1.6;font-size:0.95em;white-space:pre-wrap;animation:msgIn .25s ease}
@keyframes msgIn{from{opacity:0;transform:translateY(10px)}to{opacity:1;transform:none}}
.msg.user{background:linear-gradient(135deg,#ff8a5f,#ff6b47);color:#0a0a12;margin-left:auto;border-bottom-right-radius:4px;font-weight:500;max-width:65%}
.msg.bot{background:rgba(30,30,48,.9);color:#e0e0e0;margin-right:auto;border-bottom-left-radius:4px;border:1px solid #2a2a3e;max-width:82%}
.tools-used{font-size:0.7em;color:#c8f169;margin-bottom:10px;font-family:'SF Mono',Consolas,monospace;padding:4px 10px;background:rgba(200,241,105,.06);border-radius:8px;display:inline-block}
.comparison{margin:12px 0;border-radius:12px;overflow:hidden;border:1px solid #2a2a3e}
.comparison .no-qloo{background:rgba(255,80,80,.04);padding:12px 16px;font-size:0.85em;color:#888;border-bottom:1px solid #1a1a2e}
.comparison .no-qloo strong{color:#ff8a5f}
.comparison .with-qloo{background:rgba(200,241,105,.04);padding:12px 16px;font-size:0.85em;color:#b8cc99}
.comparison .with-qloo strong{color:#c8f169}
.brand-rec{display:inline-block;background:rgba(255,138,95,.1);border:1px solid rgba(255,138,95,.25);padding:5px 12px;border-radius:10px;margin:3px;font-size:0.82em;color:#ff8a5f;animation:pillIn .3s ease}
@keyframes pillIn{from{opacity:0;transform:scale(.85)}to{opacity:1;transform:scale(1)}}
.typing{color:#667788;padding:14px 8px;display:flex;align-items:center;gap:10px;font-style:italic}
.typing .dots{display:inline-flex;gap:5px}
.typing .dots span{width:7px;height:7px;background:#ff8a5f;border-radius:50%;animation:bounce 1.3s infinite}
.typing .dots span:nth-child(2){animation-delay:.15s}
.typing .dots span:nth-child(3){animation-delay:.3s}
@keyframes bounce{0%,60%,100%{transform:translateY(0)}30%{transform:translateY(-8px)}}
.cursor{display:inline-block;width:2px;height:1em;background:#c8f169;margin-left:2px;animation:blink .8s infinite;vertical-align:middle}
@keyframes blink{0%,50%{opacity:1}51%,100%{opacity:0}}
.input-row{display:flex;gap:10px;padding:10px 0 6px}
.input-row input{flex:1;padding:14px 20px;border-radius:24px;border:2px solid #1a1a2e;background:rgba(26,26,46,.9);color:#e0e0e0;font-size:0.95em;outline:none;transition:border-color .2s}
.input-row input:focus{border-color:#ff8a5f}
.input-row input::placeholder{color:#555}
.input-row button{padding:14px 24px;border-radius:24px;border:none;background:linear-gradient(135deg,#ff8a5f,#ff6b47);color:#0a0a12;font-weight:700;cursor:pointer;font-size:0.95em;transition:transform .15s}
.input-row button:hover{transform:scale(1.04)}
.input-row button:disabled{background:#333;cursor:not-allowed;transform:none}
footer{text-align:center;padding:8px;font-size:0.7em;color:#333}
footer a{color:#4a6a8a;text-decoration:none}
@media(max-width:768px){.container{padding:10px 12px}.msg{max-width:92%}}
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
function esc(s){return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/\n/g,'<br>')}

function addMsg(html,cls){const d=document.createElement('div');d.className='msg '+cls;d.innerHTML=html;chatbox.appendChild(d);d.scrollIntoView({behavior:'smooth',block:'end'});return d}

// Typewriter effect — reveal text character by character like ChatGPT
function typewriter(el,html,speed=12){
return new Promise(resolve=>{
// Split into text and HTML elements
const temp=document.createElement('div');temp.innerHTML=html;
let fullText=temp.textContent||'';
let i=0;
const interval=setInterval(()=>{
const partial=fullText.substring(0,i);
el.innerHTML=esc(partial)+'<span class="cursor"></span>';
el.scrollIntoView({behavior:'smooth',block:'nearest'});
i+=1;
if(i>fullText.length){
clearInterval(interval);
el.innerHTML=html; // final: show with HTML formatting
el.scrollIntoView({behavior:'smooth',block:'end'});
resolve();
}
},speed);
});
}

// Reveal brand pills one by one
function revealPills(container,pills){
return new Promise(resolve=>{
let delay=0;
pills.forEach((pill,i)=>{
setTimeout(()=>{
pill.style.display='inline-block';
pill.scrollIntoView({behavior:'smooth',block:'nearest'});
if(i===pills.length-1)resolve();
},delay);
delay+=150;
});
setTimeout(resolve,delay+300);
});
}

function showTyping(msg){
const t=document.createElement('div');t.className='typing';t.id='typing';
t.innerHTML='<span class="dots"><span></span><span></span><span></span></span> '+(msg||'Thinking…');
chatbox.appendChild(t);t.scrollIntoView({behavior:'smooth',block:'end'});
}
function hideTyping(){const t=document.getElementById('typing');if(t)t.remove()}

async function send(){
const text=input.value.trim();if(!text)return;
input.value='';
addMsg(esc(text),'user');
btn.disabled=true;

showTyping('Consulting Qloo Taste Graph…');

try{
const res=await fetch('/chat/api',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({message:text})});
const data=await res.json();
hideTyping();

// Create the bot message container
const msgEl=document.createElement('div');
msgEl.className='msg bot';
chatbox.appendChild(msgEl);

// 1. Show tools used
let toolsHTML='';
if(data.tools&&data.tools.length>0){
toolsHTML='<span class="tools-used">⚙️ '+data.tools.map(esc).join(' → ')+'</span><br>';
msgEl.innerHTML=toolsHTML;
await new Promise(r=>setTimeout(r,400));
}

// 2. Show comparison view (if present) with typewriter on the Qloo part
if(data.comparison){
const compHTML='<div class="comparison"><div class="no-qloo"><strong>❌ Without Qloo:</strong><br>'+esc(data.comparison.generic)+'</div><div class="with-qloo"><strong>✅ With Qloo Taste Graph:</strong><br>'+esc(data.comparison.qloo)+'</div></div>';
msgEl.innerHTML=toolsHTML+compHTML;
msgEl.scrollIntoView({behavior:'smooth',block:'end'});
await new Promise(r=>setTimeout(r,800));
}

// 3. Show taste-matched brands one by one
if(data.brands&&data.brands.length>0){
let pillsHTML='<div style="margin-top:10px"><em style="color:#888;font-size:0.8em">Taste-matched brands:</em><br>';
pillsHTML+='<div id="pills">';
data.brands.forEach(b=>{pillsHTML+='<span class="brand-rec" style="display:none">'+esc(b)+'</span>'});
pillsHTML+='</div></div>';
msgEl.innerHTML=toolsHTML+(data.comparison?msgEl.querySelector('.comparison')?.outerHTML||'':'')+pillsHTML;

// Reveal pills one by one
const pillEls=msgEl.querySelectorAll('.brand-rec');
for(let i=0;i<pillEls.length;i++){
await new Promise(r=>setTimeout(r,180));
pillEls[i].style.display='inline-block';
pillEls[i].style.animation='pillIn .3s ease';
msgEl.scrollIntoView({behavior:'smooth',block:'end'});
}
await new Promise(r=>setTimeout(r,300));
}

// 4. Typewriter the main reply text
const replyEl=document.createElement('div');
replyEl.style.marginTop='12px';
msgEl.appendChild(replyEl);
await typewriter(replyEl,esc(data.reply||'Sorry, try again.'),10);

}catch(e){
hideTyping();
addMsg('Connection error. Please try again.','bot');
}
btn.disabled=false;
input.focus();
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
	// Rate limit: 10 requests per minute per IP
	if !s.checkRateLimit(r.RemoteAddr) {
		json.NewEncoder(w).Encode(ChatResponse{
			Reply: "You're sending messages too quickly. Please wait a moment and try again.",
		})
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

	// Greeting/small talk — route through the LLM agent (Muse Spark)
	// so every reply is model-generated, never hardcoded.
	if isGreeting(req.Message) {
		mk := func(userID string, onImage func(string)) *agent.Agent {
			return buildWebAgent(s, s.Cfg, userID)
		}
		a := s.agentFor("web-chat", mk)
		reply, err := a.Handle(r.Context(), req.Message)
		if err != nil {
			reply = "Tell me what you love — music, films, games — and I'll match you with products from Qloo's Taste Graph!"
		}
		json.NewEncoder(w).Encode(ChatResponse{
			Reply: reply,
		})
		return
	}

	// Commerce queries → route through the agent (has katalog, bayar_paypal tools)
	if isCommerceQuery(req.Message) {
		mk := func(userID string, onImage func(string)) *agent.Agent {
			return buildWebAgent(s, s.Cfg, userID)
		}
		a := s.agentFor("web-chat", mk)
		a.Tools.CurrentRecipient = "web-chat"
		reply, err := a.Handle(r.Context(), req.Message)
		if err != nil {
			reply = "Try typing 'catalog' to browse products!"
		}
		json.NewEncoder(w).Encode(ChatResponse{
			Reply: reply,
			Tools: []string{"muse_agent", "commerce_flow"},
		})
		return
	}

	if s.Qloo != nil {
		// Extract individual taste terms from the message
		terms := extractTasteTerms(req.Message)

		var ids []string
		var entityNames []string
		for _, term := range terms {
			results, err := s.Qloo.Search(term)
			if err == nil && len(results) > 0 {
				// Take the top result that's an artist/movie/person (not an album or track)
				for _, r := range results {
					types := r.Types
					valid := false
					for _, t := range types {
						if t == "urn:entity:artist" || t == "urn:entity:movie" ||
						   t == "urn:entity:person" || t == "urn:entity:brand" ||
						   t == "urn:entity:book" || t == "urn:entity:tv_show" ||
						   t == "urn:entity:video_game" || t == "urn:entity:place" ||
						   t == "urn:entity:destination" {
							valid = true
							break
						}
					}
					if valid || len(types) == 0 {
						ids = append(ids, r.EntityID)
						entityNames = append(entityNames, r.Name)
						break
					}
				}
			}
			if len(ids) >= 3 {
				break
			}
		}

		if len(ids) > 0 {
			// Get brand recommendations from Qloo Taste Graph
			brands, err := s.Qloo.Insights("urn:entity:brand", ids)
			if err == nil && len(brands) > 0 {
				var brandNames []string
				for i, b := range brands {
					if i >= 6 {
						break
					}
					brandNames = append(brandNames, b.Name)
				}
				resp.Brands = brandNames
				resp.Reply = fmt.Sprintf("Your taste profile: %s\n\nQloo's Taste Graph analyzed your cultural preferences across 250M+ entities and found these brands that culturally resonate with what you love. This is cultural intelligence — not LLM guesswork.", joinStrings(entityNames, ", "))
				tasteSummary := joinStrings(entityNames, ", ")
				resp.Comparison = map[string]string{
					"generic": fmt.Sprintf("A generic LLM would say: \"Based on your interests in %s, you might enjoy products with smooth, sophisticated flavors...\" — it sounds plausible but it's just a guess from training data. No cultural data behind it.", tasteSummary),
					"qloo":    fmt.Sprintf("Qloo's Taste Graph (250M+ entities) maps %s to real cultural affinities: %s. This comes from actual taste correlation data — the same graph that powers recommendations for major global brands.", tasteSummary, joinStrings(brandNames, ", ")),
				}
			} else {
				resp.Reply = "I found your taste profile, but couldn't get brand recommendations. Try mentioning specific artists, films, or genres you love!"
			}
		} else {
			resp.Reply = "Tell me what you like — specific artists, films, books, or hobbies — so I can match you with products from the taste graph!"
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


// extractTasteTerms pulls out individual taste descriptors from a sentence.
// "I love Miles Davis, jazz, and French noir films" → ["Miles Davis", "jazz", "French noir films"]
func extractTasteTerms(message string) []string {
	stop := map[string]bool{
		"i": true, "love": true, "like": true, "enjoy": true, "into": true,
		"and": true, "the": true, "a": true, "an": true, "my": true,
		"what": true, "recommend": true, "suits": true, "taste": true,
		"films": true, "movies": true, "music": true, "brands": true,
		"coffee": true, "for": true, "me": true, "some": true, "suit": true,
		"would": true, "should": true, "get": true, "show": true,
	}
	
	// Split by common delimiters
	var terms []string
	current := ""
	for _, ch := range message {
		if ch == ',' || ch == '.' || ch == '!' || ch == '?' || ch == ';' {
			if current != "" {
				terms = append(terms, current)
			}
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		terms = append(terms, current)
	}

	// Process each term — remove stop words, keep meaningful phrases
	var results []string
	for _, t := range terms {
		words := strings.Fields(t)
		var meaningful []string
		for _, w := range words {
			lw := strings.ToLower(strings.TrimSpace(w))
			if !stop[lw] {
				meaningful = append(meaningful, w)
			}
		}
		if len(meaningful) > 0 {
			results = append(results, strings.Join(meaningful, " "))
		}
	}
	return results
}


// isGreeting returns true for short messages that are clearly greetings
// or not taste-related queries — skip Qloo for these.
func isGreeting(msg string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(msg))
	if len(trimmed) < 3 {
		return true
	}
	greetings := map[string]bool{
		"hi": true, "hello": true, "hey": true, "yo": true,
		"halo": true, "hai": true, "sup": true, "what's up": true,
		"hi there": true, "hello there": true, "test": true,
	}
	if greetings[trimmed] {
		return true
	}
	// If message has no meaningful taste terms, treat as conversation
	terms := extractTasteTerms(msg)
	if len(terms) == 0 {
		return true
	}
	return false
}


// isCommerceQuery detects if the user wants to browse/buy/sell (agent handles it)
func isCommerceQuery(msg string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(msg))
	keywords := []string{
		"catalog", "catalogue", "show products", "browse", "shop", "store",
		"buy", "purchase", "checkout", "order", "sell", "add product",
		"katalog", "beli", "tambah produk", "jual",
		"invoice", "paypal", "pay with", "how much",
	}
	for _, kw := range keywords {
		if strings.Contains(trimmed, kw) {
			return true
		}
	}
	return false
}
