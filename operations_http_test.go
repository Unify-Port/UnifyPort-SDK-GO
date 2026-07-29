package unifyport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGeneratedOptionalBodyAndEnvelopeMapping(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/accounts/acc_test/auth/cancel" {
			t.Errorf("unexpected request target: %s %s", request.Method, request.URL.Path)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if len(body) != 0 {
			t.Errorf("optional body was not omitted: %q", body)
		}
		response.Header().Set("Content-Type", "application/json")
		response.Header().Set("X-Request-Id", "req_header")
		_, _ = io.WriteString(response, `{"request_id":"req_envelope","client_request_id":"client_request","data":{"status":"canceled"}}`)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:           server.URL,
		APIKeyProvider:    StaticAPIKey("test-key"),
		HTTPClient:        server.Client(),
		AllowInsecureHTTP: true,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	result, err := client.CancelAccountAuth(context.Background(), CancelAccountAuthInput{AccountID: "acc_test"})
	if err != nil {
		t.Fatalf("CancelAccountAuth() error = %v", err)
	}
	if result.StatusCode != http.StatusOK || result.RequestID != "req_envelope" || result.ClientRequestID != "client_request" {
		t.Fatalf("unexpected result metadata: %#v", result)
	}
	if result.Data["status"] != "canceled" {
		t.Fatalf("unexpected result data: %#v", result.Data)
	}
}

func TestGeneratedParserRejectsUndeclaredSuccessStatus(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(response, `{"request_id":"req_unexpected","data":{"id":"sensitive-response-value"}}`)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:           server.URL,
		APIKeyProvider:    StaticAPIKey("test-key"),
		HTTPClient:        server.Client(),
		AllowInsecureHTTP: true,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.GetWorkspace(context.Background(), WithoutRetry())
	var parseError *ResponseParseError
	if !errors.As(err, &parseError) || parseError.StatusCode != http.StatusAccepted || parseError.OperationID != "getWorkspace" {
		t.Fatalf("unexpected error: %#v", err)
	}
	if strings.Contains(err.Error(), "sensitive-response-value") {
		t.Fatalf("response content leaked through error: %v", err)
	}
}

func TestMarkConversationReadValidatesDependentRequiredBeforeExternalWork(t *testing.T) {
	t.Parallel()
	var providerCalls atomic.Int32
	var requestCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestCalls.Add(1)
		if request.Method != http.MethodPost || request.URL.Path != "/v1/accounts/acc_test/conversations/read" {
			t.Errorf("unexpected request target: %s %s", request.Method, request.URL.Path)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response, `{"data":{"ok":true}}`)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL: server.URL,
		APIKeyProvider: func(context.Context) (string, error) {
			providerCalls.Add(1)
			return "synthetic-test-key", nil
		},
		HTTPClient:        server.Client(),
		AllowInsecureHTTP: true,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if _, err := client.MarkConversationRead(context.Background(), MarkConversationReadInput{
		AccountID: "acc_test",
		Body:      MarkConversationReadRequestBody{ConversationID: "conv_test"},
	}); err != nil {
		t.Fatalf("MarkConversationRead() without a message boundary error = %v", err)
	}
	messageID, senderID := "msg_test", "sender_test"
	if _, err := client.MarkConversationRead(context.Background(), MarkConversationReadInput{
		AccountID: "acc_test",
		Body: MarkConversationReadRequestBody{
			ConversationID:      "conv_test",
			UpToMessageID:       &messageID,
			UpToMessageSenderID: &senderID,
		},
	}); err != nil {
		t.Fatalf("MarkConversationRead() with a complete message boundary error = %v", err)
	}
	if providerCalls.Load() != 2 || requestCalls.Load() != 2 {
		t.Fatalf("valid calls did not reach credential and HTTP exactly once each: provider=%d HTTP=%d", providerCalls.Load(), requestCalls.Load())
	}

	invalidBodies := []MarkConversationReadRequestBody{
		{ConversationID: "conv_test", UpToMessageID: &messageID},
		{ConversationID: "conv_test", UpToMessageSenderID: &senderID},
	}
	for _, body := range invalidBodies {
		providerBefore, requestsBefore := providerCalls.Load(), requestCalls.Load()
		_, callErr := client.MarkConversationRead(context.Background(), MarkConversationReadInput{
			AccountID: "acc_test",
			Body:      body,
		})
		var configurationError *ConfigurationError
		if !errors.As(callErr, &configurationError) || configurationError.Field != "MarkConversationReadInput.Body" {
			t.Fatalf("incomplete message boundary error = %#v, want ConfigurationError", callErr)
		}
		// 本地契约错误不得解析 credential 或创建网络请求，避免无效输入产生外部副作用。
		if providerCalls.Load() != providerBefore || requestCalls.Load() != requestsBefore {
			t.Fatalf("invalid call reached external work: provider=%d HTTP=%d", providerCalls.Load(), requestCalls.Load())
		}
	}
}
