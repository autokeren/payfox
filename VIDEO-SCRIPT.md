# PayFox Demo Video — Recording Guide & Script

## VIDEO REQUIREMENTS (from hackathon rules)
- ⏱️ UNDER 3 MINUTES (judges stop watching at 3:00)
- 📱 Shows project working END-TO-END on Messenger (phone screen)
- 🌐 YouTube, public
- 🚫 No copyrighted music
- 🗣️ English narration (judges are PayPal devs)

---

## RECORDING SETUP

**What you need:**
- Phone with Messenger app (logged into a Facebook account that can message the page)
- Screen recorder (iPhone: Control Center → Screen Record / Android: Quick Settings → Screen Record)
- Quiet room for voiceover
- PayPal sandbox buyer account ready (login: sb-buyer / password from your PayPal dev dashboard)

**BEFORE RECORDING — prep the demo:**
1. DELETE the Messenger conversation with the page (so the Welcome message appears fresh)
2. Have PayPal sandbox credentials ready to log in quickly
3. Phone brightness: MAX (recording washes out)
4. Do Not Disturb: ON (no notification popups!)

---

## THE SCRIPT (2:45 total)

### SCENE 1 — HOOK (0:00 – 0:15) — title card
**Show:** Black screen, white text:
```
PayFox 🦊
AI Commerce in Facebook Messenger
Built with PayPal + Muse Spark AI
```

**Narrate (15s):**
> "Five hundred million people use Messenger every month, and millions of small businesses sell through chat. But taking payment? Still bank numbers, screenshots, and hope. What if the entire store — catalog, checkout, payment, receipt — lived inside one chat?"

---

### SCENE 2 — LIVE DEMO (0:15 – 2:15) — phone screen recording

**Show:** Start recording → open Messenger → open the PayFox page chat

**Narrate:**
> "This is PayFox — an AI commerce agent living inside Facebook Messenger."

**[Welcome message appears]**

> "I just opened the chat, and the agent already greeted me."

**[Send a VOICE NOTE — say: "show me the catalog"]** ← KEY MOMENT #1

> "Now, instead of typing, I'm sending a voice note."
> *(wait for transcription + typing indicator)*
> "Muse Spark transcribes my voice, and PayFox responds —"

**[Product cards appear with photos + Buy buttons]**

> "— with a full product catalog. Photos, descriptions, prices, and a Buy button on every card."

**[Tap 🛒 Buy Now on Kopi Luwak]**

> "I'll buy the Kopi Luwak. One tap."

**[Order confirmation + PayPal link appears in chat]**

> "PayFox created a PayPal order through the Orders API and sent me the payment link with my order number."

**[Tap the PayPal link → PayPal checkout opens in browser]**

> "One tap takes me straight to PayPal checkout." ← KEY MOMENT #2

**[Log into sandbox → tap Pay]**

> "I'll pay with the sandbox buyer account — in production this would be a card, bank, or PayPal balance."

**[Payment completes → switch back to Messenger]**

**[RECEIPT CARD appears automatically]**

> "Now, here's the magic. I didn't refresh. I didn't type anything. PayPal fired a webhook, PayFox received it, and — my receipt appeared. My name, the product photo, the order number, the total — confirmed automatically in chat." ← KEY MOMENT #3 (THE WOW)

---

### SCENE 3 — TECH (2:15 – 2:35) — architecture slide
**Show:** The flow diagram from README (make a clean PNG slide):

```
Customer → PayFox Agent → PayPal
   voice/catalog → Muse Spark → Orders API v2
   tap Buy      → order+link → checkout page
   pays         → WEBHOOK    → receipt card in chat
```

**Narrate (20s):**
> "Under the hood: Muse Spark 1.3 from the Meta Model API drives the agent — it routes intents, calls tools, transcribes voice, and reads images natively. PayPal handles the money: the Orders API for checkout, OAuth2 for security, and webhooks for real-time payment confirmation. The entire loop is autonomous — no human in the middle."

---

### SCENE 4 — CLOSE (2:35 – 2:45) — outro card
**Show:**
```
PayFox 🦊
The store that comes to you.

github.com/autokeren/payfox
Muse Spark 1.3 · PayPal Orders API · Webhooks
```

**Narrate (10s):**
> "PayFox turns any Facebook Page into an AI store in under a minute. Voice note to purchase — commerce with zero friction. Full source code is open source — link in the description."

---

## EDITING CHECKLIST
- [ ] Trim dead air (silence between steps)
- [ ] Speed up the PayPal login typing (2x, no one needs to watch typing)
- [ ] Captions/subtitles: YES — add burned-in subtitles (judges may watch muted!)
- [ ] Title card: 3-4 seconds each
- [ ] Total: under 2:50 (leave buffer)
- [ ] Export: 1080p, 30fps

## UPLOAD
- Title: "PayFox — AI Commerce Agent in Facebook Messenger (PayPal + Muse Spark)"
- Description: problem, solution, repo link, tech stack
- Tags: paypal, ai, hackathon, messenger, commerce, agent
