# PayFox 🦊

**An AI commerce agent that lives in Facebook Messenger. Customers send a voice note, browse a product catalog, tap "Buy", and pay with PayPal — all inside the chat. Payments confirm automatically.**

Built for the [PayPal AI Hackathon](https://paypalaihackathon.devpost.com/).

## The problem

Over **500 million people** use Facebook Messenger every month. Millions of small businesses sell through chat — but taking payment means sending bank account numbers, waiting for transfer screenshots, and manual reconciliation. That's friction, fraud risk, and lost sales.

## The solution

PayFox turns any Facebook Page into an AI-powered store:

```
Customer                    PayFox Agent                    PayPal
   │                            │                            │
   │── voice note: "catalog" ──▶│                            │
   │◀──── product cards 🛍️ ─────│                            │
   │                            │                            │
   │── tap "🛒 Buy Now" ───────▶│                            │
   │                            │── create order (API) ─────▶│
   │◀──── payment link 💳 ──────│◀──── approve link ─────────│
   │                            │                            │
   │── taps link, pays ─────────│◀───────────────────────────│
   │                            │◀── WEBHOOK: APPROVED ──────│
   │                            │── auto-CAPTURE (API) ──────▶│  ← money moves
   │                            │◀── WEBHOOK: COMPLETED ─────│
   │◀── receipt card 🧾 ────────│                            │
```

**One voice note → product cards → one tap → paid → receipt. Automatically.**

## The flow (end-to-end)

1. **Discover** — new customer opens the chat → automatic welcome message
2. **Browse** — voice note or type `catalog` → product cards with photos, prices, and **Buy** buttons (Messenger Generic Template)
3. **Buy** — tap **🛒 Buy Now** → PayFox creates a PayPal order (Orders API v2) and sends the payment link with the order number
4. **Pay** — customer taps the link → pays on PayPal (card, bank, or balance)
5. **Auto-capture** — PayPal webhook `CHECKOUT.ORDER.APPROVED` → PayFox captures the payment → money moves
6. **Receipt** — PayPal webhook `PAYMENT.CAPTURE.COMPLETED` → PayFox sends a **receipt card** (Messenger Receipt Template) with the customer's name, product photo, order number, and total — automatically in chat

Store owners manage everything conversationally: *"add product: Gayo Coffee 1kg, $35"* → the catalog updates instantly.

## What's inside

| Component | What it does |
|---|---|
| **Muse Spark 1.3** (Meta Model API) | The reasoning brain — routes intents, calls tools, transcribes voice, reads images natively, writes every reply |
| **PayPal Orders API v2** | Creates checkout orders, auto-captures payments |
| **PayPal Webhooks** | `CHECKOUT.ORDER.APPROVED` → auto-capture. `PAYMENT.CAPTURE.COMPLETED` → receipt card in chat |
| **Messenger Platform** | Product cards (Generic Template), Buy buttons (postbacks), receipt cards (Receipt Template), typing indicators, voice note transcription |
| **Muse Image** | Generates product photos for the catalog |
| **Order tracking** | Every checkout maps `order_id → Messenger user`; persists to disk so webhooks survive restarts |
| **Product catalog** | JSON-persisted; owners manage by chatting; images served from the workspace |

## PayPal APIs used

- **Orders v2** — `POST /v2/checkout/orders` (create), `POST /v2/checkout/orders/{id}/capture` (auto-capture on approval)
- **OAuth2** — client credentials token management
- **Webhooks** — live-registered; `CHECKOUT.ORDER.APPROVED` triggers capture, `PAYMENT.CAPTURE.COMPLETED` triggers the receipt
- **Invoicing v2** — `paypal_invoice` tool for formal B2B orders

## Design notes

- **Zero hardcoded replies.** Every sentence the customer reads — acknowledgments, confirmations, errors — is written by Muse Spark. The quick *"On it!"* ack comes from a parallel intent-router request to the model.
- **Voice-first.** Voice notes are downloaded, transcoded, and sent through Muse Voice Transcribe. "Show me the catalog" by voice works exactly like typing.
- **The tool-routing fix.** Early versions confused *show catalog* with *add product* — the model picked the wrong tool 100% of the time from real users. Fix: tool descriptions with explicit routing rules + a system prompt that maps intent → tool name. Result: 100% correct routing in E2E tests.
- **The catalog singleton bug.** Two catalog instances (one for the agent, one for the webhook handler) meant every Buy button tap returned "product not found" — the agent added products to one catalog, the BUY button read from a stale second one. Fix: one shared catalog instance across the entire process.
- **The auto-capture lesson.** PayPal Orders API is two-step: customer approves, *then the merchant captures*. Without the capture call, money stays in limbo even after the customer pays. PayFox auto-captures on the `APPROVED` webhook.
- **Order tracking survives restarts.** `orders.json` maps every PayPal order to the Messenger user who bought, so a webhook arriving after a server restart still finds the buyer and sends the receipt.

## Quick start

```bash
go build -o payfox .

# Configure (~/.payfox/config.yaml):
#   meta_api_key:            your Meta Model API key
#   messenger.access_token:  your Page Access Token
#   messenger.verify_token:  any string
#   workspace:               data directory
#   public_url:              your HTTPS webhook URL (e.g. Cloudflare Tunnel)

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

Full config reference: `config.example.yaml`

## Demo video

[YouTube — PayFox demo](https://youtube.com/TODO)

## License

MIT
