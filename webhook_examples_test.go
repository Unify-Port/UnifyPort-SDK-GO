package unifyport

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestPublicWebhookExamplesPreserveEveryField(t *testing.T) {
	// 同一批公开样例覆盖 false、0、空字符串与稀疏 call update，避免 omitempty 静默丢字段。
	payloads, err := os.ReadFile("testdata/webhook-examples.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name   string          `json:"name"`
		Schema string          `json:"schema"`
		Body   json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(payloads, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 49 {
		t.Fatalf("example coverage: %d", len(cases))
	}
	factories := map[string]func() any{
		"AccountProfileUpdatedWebhookEvent":                   func() any { return new(AccountProfileUpdatedWebhookEvent) },
		"AccountStartedWebhookEvent":                          func() any { return new(AccountStartedWebhookEvent) },
		"CallMessageReceivedWebhookEvent":                     func() any { return new(CallMessageReceivedWebhookEvent) },
		"CallMessageUpdatedWebhookEvent":                      func() any { return new(CallMessageUpdatedWebhookEvent) },
		"ContactUpdatedWebhookEvent":                          func() any { return new(ContactUpdatedWebhookEvent) },
		"ConversationDisappearingMessagesUpdatedWebhookEvent": func() any { return new(ConversationDisappearingMessagesUpdatedWebhookEvent) },
		"ConversationHistoryWebhookEvent":                     func() any { return new(ConversationHistoryWebhookEvent) },
		"MessageCommerceHistoryWebhookEvent":                  func() any { return new(MessageCommerceHistoryWebhookEvent) },
		"MessageCommerceReceivedWebhookEvent":                 func() any { return new(MessageCommerceReceivedWebhookEvent) },
		"MessageCommerceUpdatedWebhookEvent":                  func() any { return new(MessageCommerceUpdatedWebhookEvent) },
		"MessageInteractiveHistoryWebhookEvent":               func() any { return new(MessageInteractiveHistoryWebhookEvent) },
		"MessageInteractiveReceivedWebhookEvent":              func() any { return new(MessageInteractiveReceivedWebhookEvent) },
		"MessageInteractiveUpdatedWebhookEvent":               func() any { return new(MessageInteractiveUpdatedWebhookEvent) },
		"MessageLinkPreviewHistoryWebhookEvent":               func() any { return new(MessageLinkPreviewHistoryWebhookEvent) },
		"MessageLinkPreviewReceivedWebhookEvent":              func() any { return new(MessageLinkPreviewReceivedWebhookEvent) },
		"MessageLinkPreviewUpdatedWebhookEvent":               func() any { return new(MessageLinkPreviewUpdatedWebhookEvent) },
		"MessagePinUpdatedWebhookEvent":                       func() any { return new(MessagePinUpdatedWebhookEvent) },
		"MessagePollHistoryWebhookEvent":                      func() any { return new(MessagePollHistoryWebhookEvent) },
		"MessagePollReceivedWebhookEvent":                     func() any { return new(MessagePollReceivedWebhookEvent) },
		"MessagePollVoteUpdatedWebhookEvent":                  func() any { return new(MessagePollVoteUpdatedWebhookEvent) },
		"MessageReferralHistoryWebhookEvent":                  func() any { return new(MessageReferralHistoryWebhookEvent) },
		"MessageReferralReceivedWebhookEvent":                 func() any { return new(MessageReferralReceivedWebhookEvent) },
		"MessageReferralUpdatedWebhookEvent":                  func() any { return new(MessageReferralUpdatedWebhookEvent) },
		"WhatsAppAccountAuthFailedWebhookEvent":               func() any { return new(WhatsAppAccountAuthFailedWebhookEvent) },
		"WhatsAppMessageDeviceWebhookEvent":                   func() any { return new(WhatsAppMessageDeviceWebhookEvent) },
	}
	for _, example := range cases {
		t.Run(example.Name, func(t *testing.T) {
			factory, ok := factories[example.Schema]
			if !ok {
				t.Fatal("missing public schema type")
			}
			target := factory()
			if err := json.Unmarshal(example.Body, target); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(target)
			if err != nil {
				t.Fatal(err)
			}
			decode := func(body []byte) any {
				var value any
				decoder := json.NewDecoder(bytes.NewReader(body))
				decoder.UseNumber()
				if err := decoder.Decode(&value); err != nil {
					t.Fatal(err)
				}
				var normalize func(any) any
				normalize = func(value any) any {
					switch typed := value.(type) {
					case string:
						if parsed, err := time.Parse(time.RFC3339Nano, typed); err == nil {
							return parsed.Format(time.RFC3339Nano)
						}
					case map[string]any:
						for key, item := range typed {
							typed[key] = normalize(item)
						}
					case []any:
						for index, item := range typed {
							typed[index] = normalize(item)
						}
					}
					return value
				}
				return normalize(value)
			}
			if !reflect.DeepEqual(decode(example.Body), decode(encoded)) {
				t.Fatalf("webhook fields changed: %s", encoded)
			}
		})
	}
}
