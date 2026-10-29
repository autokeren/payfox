package messenger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Event is a single incoming message extracted from a webhook payload.
type Event struct {
	SenderID string
	RecipientID string
	Text     string
	Attachments []Attachment
}

type Attachment struct {
	Type      string `json:"type"`
	PayloadURL string `json:"payload_url"`
}

type webhookPayload struct {
	Entry []struct {
		Messaging []struct {
			Sender    struct{ ID string `json:"id"` } `json:"sender"`
			Recipient struct{ ID string `json:"id"` } `json:"recipient"`
			Message   struct {
				MID        string `json:"mid"`
				Text       string `json:"text"`
				Attachments []struct {
					Type    string `json:"type"`
					Payload struct {
						URL string `json:"url"`
					} `json:"payload"`
				} `json:"attachments"`
			} `json:"message"`
		} `json:"messaging"`
	} `json:"entry"`
}

// ParseWebhook decodes the Messenger webhook body into simple events.
func ParseWebhook(body io.Reader) ([]Event, error) {
	var p webhookPayload
	if err := json.NewDecoder(body).Decode(&p); err != nil {
		return nil, err
	}
	var out []Event
	for _, entry := range p.Entry {
		for _, m := range entry.Messaging {
			ev := Event{
				SenderID:    m.Sender.ID,
				RecipientID: m.Recipient.ID,
				Text:       m.Message.Text,
			}
			for _, a := range m.Message.Attachments {
				ev.Attachments = append(ev.Attachments, Attachment{Type: a.Type, PayloadURL: a.Payload.URL})
			}
			if ev.Text == "" && len(ev.Attachments) == 0 {
				continue
			}
			out = append(out, ev)
		}
	}
	return out, nil
}

// Sender sends text replies via the Messenger Send API.
type Sender struct {
	token string
	http  *http.Client
}

func NewSender(token string) *Sender {
	return &Sender{token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

func (s *Sender) Send(recipient, text string) error {
	if s.token == "" {
		return fmt.Errorf("no page access token")
	}
	payload, _ := json.Marshal(map[string]any{
		"recipient": map[string]any{"id": recipient},
		"message":   map[string]any{"text": text},
	})
	req, err := http.NewRequest(http.MethodPost, "https://graph.facebook.com/v21.0/me/messages", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	q := req.URL.Query()
	q.Set("access_token", s.token)
	req.URL.RawQuery = q.Encode()
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Action sends a sender_action (typing_on / typing_off / mark_seen).
func (s *Sender) Action(recipient, action string) error {
	if s.token == "" {
		return fmt.Errorf("no page access token")
	}
	payload, _ := json.Marshal(map[string]any{
		"recipient":    map[string]any{"id": recipient},
		"sender_action": action,
	})
	req, err := http.NewRequest(http.MethodPost, "https://graph.facebook.com/v21.0/me/messages", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	q := req.URL.Query()
	q.Set("access_token", s.token)
	req.URL.RawQuery = q.Encode()
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// SendImage sends a native image attachment (Facebook fetches the URL once
// and stores it — displays as a regular image bubble in the chat).
func (s *Sender) SendImage(recipient, imageURL string) error {
	if s.token == "" {
		return fmt.Errorf("no page access token")
	}
	payload, _ := json.Marshal(map[string]any{
		"recipient": map[string]any{"id": recipient},
		"message": map[string]any{
			"attachment": map[string]any{
				"type": "image",
				"payload": map[string]any{
					"url":        imageURL,
					"is_reusable": true,
				},
			},
		},
	})
	req, err := http.NewRequest(http.MethodPost, "https://graph.facebook.com/v21.0/me/messages", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	q := req.URL.Query()
	q.Set("access_token", s.token)
	req.URL.RawQuery = q.Encode()
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send image %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	return nil
}

// DownloadAttachment fetches an attachment payload URL (Facebook CDN) and
// returns the raw bytes. Audio (voice notes) arrive as .mp4/.m4a; images as .jpg/.png.
func DownloadAttachment(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
