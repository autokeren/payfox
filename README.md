# PayFox 🦊

**An AI commerce agent that lives in Facebook Messenger. Customers browse a product catalog, tap "Buy", and pay with PayPal — all inside the chat. Payments confirm automatically.**

Built for the [PayPal AI Hackathon](https://paypalaihackathon.devpost.com/) — 100% PayPal + AI. No humans in the loop.

## The problem

Over **500 million people** use Facebook Messenger every month. Small businesses in Southeast Asia and beyond sell through chat — but taking payment means sending bank account numbers, screenshots of transfers, and manual reconciliation. That's friction, fraud risk, and lost sales.

## The solution

PayFox turns any Facebook Page into an AI-powered store in under a minute:

```
Customer                    PayFox Agent                    PayPal
   │                            │                            │
   │── voice note / "catalog" ─▶│                            │
   │◀──── product cards 🛍️ ─────│                            │
   │                            │                            │
   │── tap "🛒 Buy Now" ──────▶│                            │
   │                            │── create order (API) ─────▶│
   │◀──── payment link 💳 ──────│◀──── approve link ─────────│
   │                            │                            │
   │── taps link, pays ─────────│◀───✊───────────────────────│
   │                            │                            │
   │                            │◀── WEBHOOK: PAID ─────────│
   │◀── "🎉 Payment received!" ─│                            │
```

**One voice note → product cards → one tap → paid.**

## What's inside

| Component | What it does |
|---|---|
| **Muse Spark 1.3** (Meta Model API) | The reasoning brain — decides which tool to call, writes all replies, routes intents |
| **PayPal Orders API v2** | Creates checkout orders, returns approve links |
| **PayPal Webhooks** | `PAYMENT.CAPTURE.COMPLETED` → PayFox confirms the order in chat automatically |
| **Messenger Platform** | Product cards (Generic Template), Buy buttons (postbacks), typing indicators, voice note transcription |
| **Muse Image** | Generates product photos for the catalog |
| **Product catalog** | JSON-persisted, survives restarts; store owners manage it by chatting |
| **Order tracking** | Every checkout maps `order_id → Messenger user` so webhooks find who to notify |

## The flow (end-to-end)

1. **Discover** — new customer opens the chat → automatic welcome message
2. **Browse** — types `catalog` (or voice notes it) → product cards with photos, prices, and **Buy** buttons
3. **Buy** — taps **🛒 Buy Now** → PayFox creates a PayPal order and sends the payment link with the order ID
4. **Pay** — customer taps the link → pays on PayPal (card, bank, or balance)
5. **Confirm** — PayPal fires a webhook → PayFox maps the order to the customer → sends **"🎉 Payment SUCCESSFUL!"** with the receipt — all in chat, automatically

Store owners manage everything conversationally: *"add product: Gayo Coffee 1kg, $35"* → the catalog updates instantly.

## Quick start

```bash
go build -o payfox .

# Configure (~/.payfox/config.yaml):
#   meta_api_key:     your Meta Model API key
#   messenger.access_token:  your Page Access Token
#   messenger.verify_token: any string
#   workspace:        data directory
#   public_url:       your HTTPS webhook URL (e.g. Cloudflare Tunnel)

export PAYPAL_CLIENT_ID=xxx
export PAYPAL_CLIENT_SECRET=xxx   # sandbox by default; PAYPAL_LIVE=1 for production

./payfox serve
```

Then in your Facebook App:
- Webhook callback: `https://YOUR-DOMAIN/webhook/messenger`
- Subscribe to: `messages`, `messaging_postbacks`

For PayPal payment confirmations, register a webhook at [developer.paypal.com/dashboard/webhooks](https://developer.paypal.com/dashboard/webhooks):
- URL: `https://YOUR-DOMAIN/webhook/paypal`
- Events: `CHECKOUT.ORDER.APPROVED`, `PAYMENT.CAPTURE.COMPLETED`

> Full config reference: `config.example.yaml`

## Design notes

- **Zero hardcoded replies.** Every sentence the customer reads — acknowledgments, confirmations, errors — is written by Muse Spark in the customer's language. The quick *"On it!"* ack comes from a parallel intent-router request to the model.
- **Voice-first.** Voice notes are downloaded, transcoded (ffmpeg), and sent through Muse Voice Transcribe. "Kopi luwak satu" by voice works exactly like typing.
- **The tool routing problem.** Early versions confused *show catalog* with *add product* — the model picked the wrong tool. Fix: tool descriptions with explicit routing rules + a system prompt that maps user intent → tool name. The result: 100% correct routing in E2E tests.
- **Order tracking survives restarts.** `orders.json` maps every PayPal order to the Messenger user who bought, so a webhook that arrives after a server restart still finds the buyer.
- **Singleton catalog.** One catalog instance shared by all agent sessions and the webhook handler — otherwise the BUY button reads a stale catalog and fails with "product not found". This bug took a full E2E pass to find.

## PayPal APIs used

- **Orders v2** — create order, get approve link (`POST /v2/checkout/orders`)
- **OAuth2** — client credentials token management
- **Webhooks** — registered live; `CHECKOUT.ORDER.APPROVED`, `PAYMENT.CAPTURE.COMPLETED`, `CHECKOUT.ORDER.COMPLETED` → automatic in-chat confirmation
- **Invoicing v2** — `paypal_invoice` tool for formal B2B orders (draft invoice + payment link)

## Demo video

[YouTube — PayFox demo](https://youtube.com/TODO)

## License

MIT
