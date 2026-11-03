package unifyport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestFacebookMetadataPrecisionAndSparseValues(t *testing.T) {
	// 保留原生小数精度以及显式 false/0；不能用 float32 或缺省值掩盖媒体状态。
	var metadata FacebookAttachmentMetadata
	if err := json.Unmarshal([]byte(`{"width":16777217,"duration":123456.789012345,"animated":false,"ptt":false}`), &metadata); err != nil {
		t.Fatal(err)
	}
	var duration float64 = *metadata.Duration
	if duration != 123456.789012345 || *metadata.Width != 16777217 || *metadata.Animated || *metadata.Ptt {
		t.Fatal("Facebook metadata lost precision or explicit false")
	}
	var provider ProviderName = "facebook"
	var legacy ProviderName = "whatsapp-protocol"
	if !provider.Valid() || !legacy.Valid() {
		t.Fatal("provider enum lost new or legacy channels")
	}
	var direction FacebookMessageDirection = "outbound"
	if !direction.Valid() {
		t.Fatal("outbound self-sync was rejected")
	}
	direction = "unknown"
	if direction.Valid() {
		t.Fatal("direction enum was widened")
	}
}

func TestFacebookCreationRetainsSessionAndNoRetry(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		var body struct {
			Provider string `json:"provider"`
			AuthMode string `json:"auth_mode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Provider != "facebook" || body.AuthMode != "session" || r.Header.Get("Idempotency-Key") != "facebook-example" {
			t.Error("Facebook request was rewritten")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, `{"error":{"code":"provider_unavailable"}}`)
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, APIKeyProvider: StaticAPIKey("test-key"), HTTPClient: server.Client(), AllowInsecureHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	key := "facebook-example"
	_, err = client.CreateAccount(context.Background(), CreateAccountInput{Params: &CreateAccountParams{IdempotencyKey: &key}, Body: CreateAccountRequestBody{Provider: "facebook", AuthMode: "session", Region: "sg"}})
	var apiError *APIError
	if !errors.As(err, &apiError) || attempts.Load() != 1 {
		t.Fatal("uncertain creation must not be retried")
	}
}
