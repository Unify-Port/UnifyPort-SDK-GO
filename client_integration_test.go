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
	"time"
)

func TestClientFacadeAppliesPolicyAndPreservesSuccessEnvelope(t *testing.T) {
	var readAttempts atomic.Int32
	var writeAttempts atomic.Int32
	var providerCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Api-Key") != "synthetic-test-key" {
			t.Errorf("unexpected API key header")
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1/workspace":
			if readAttempts.Add(1) == 1 {
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusServiceUnavailable)
				_, _ = io.WriteString(writer, `{"error":{"code":"TEMPORARY","message":"must-not-leak"}}`)
				return
			}
			writer.Header().Set("Content-Type", `Application/JSON; charset="utf-8"; boundary="private-value"`)
			writer.Header().Set("Set-Cookie", "session=must-not-leak")
			writer.Header().Set("X-Request-Id", "req_header")
			_, _ = io.WriteString(writer, `{"request_id":"req_body","client_request_id":"client_123","data":{"name":"example","status":"active","account_quota":18446744073709551615,"account_quota_unlimited":false}}`)
		case request.Method == http.MethodPatch && request.URL.Path == "/v1/workspace":
			writeAttempts.Add(1)
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(writer, `{"request_id":"req_write","error":{"code":"UPSTREAM_BUSY","message":"private upstream detail"}}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/v1/accounts/acc_test":
			writer.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL: server.URL,
		APIKeyProvider: func(context.Context) (string, error) {
			providerCalls.Add(1)
			return "synthetic-test-key", nil
		},
		// nil HTTPClient 覆盖 typed-nil interface 的回归路径，并使用 SDK 安全默认值。
		AllowInsecureHTTP: true,
		Retry: &RetryConfig{
			MaxRetries: 2,
			BaseDelay:  time.Nanosecond,
			MaxDelay:   time.Nanosecond,
		},
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	workspace, err := client.GetWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetWorkspace() error = %v", err)
	}
	if readAttempts.Load() != 2 {
		t.Fatalf("retryable read attempts = %d, want 2", readAttempts.Load())
	}
	if providerCalls.Load() != 1 {
		t.Fatalf("API key provider calls = %d, want 1 for one logical call", providerCalls.Load())
	}
	if (workspace.Data.AccountQuota == nil || *workspace.Data.AccountQuota != ^uint64(0)) || workspace.Data.Name != "example" {
		t.Fatalf("workspace data lost precision or fields: %#v", workspace.Data)
	}
	if workspace.StatusCode != http.StatusOK || workspace.RequestID != "req_body" || workspace.ClientRequestID != "client_123" {
		t.Fatalf("success envelope metadata was not preserved: %#v", workspace)
	}
	if workspace.Headers.Get("Content-Type") != "application/json" || workspace.Headers.Get("Set-Cookie") != "" {
		t.Fatalf("response headers were not safely filtered: %#v", workspace.Headers.Clone())
	}

	_, err = client.UpdateWorkspace(context.Background(), UpdateWorkspaceInput{
		Body: UpdateWorkspaceRequestBody{Name: "example", Status: "active"},
	})
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.StatusCode != http.StatusServiceUnavailable || apiError.Code != "UPSTREAM_BUSY" || apiError.RequestID != "req_write" {
		t.Fatalf("unexpected write error: %#v", err)
	}
	if writeAttempts.Load() != 1 {
		t.Fatalf("non-retryable write attempts = %d, want 1", writeAttempts.Load())
	}
	if providerCalls.Load() != 2 {
		t.Fatalf("API key provider calls = %d, want once per logical call", providerCalls.Load())
	}
	if strings.Contains(err.Error(), "private upstream detail") {
		t.Fatalf("API error leaked upstream message: %v", err)
	}

	deleted, err := client.DeleteAccount(context.Background(), DeleteAccountInput{AccountID: "acc_test"})
	if err != nil {
		t.Fatalf("DeleteAccount() error = %v", err)
	}
	if deleted.StatusCode != http.StatusNoContent || deleted.Data != nil {
		t.Fatalf("unexpected no-content result: %#v", deleted)
	}
}

func TestClientFacadeRejectsMalformedSuccessWithoutLeakingBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-Id", "req_parse")
		_, _ = io.WriteString(writer, `{"data":{"name":"private-response-fragment"}`)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:           server.URL,
		APIKeyProvider:    StaticAPIKey("synthetic-test-key"),
		AllowInsecureHTTP: true,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.GetWorkspace(context.Background(), WithoutRetry())
	var parseError *ResponseParseError
	if !errors.As(err, &parseError) || parseError.StatusCode != http.StatusOK || parseError.RequestID != "req_parse" {
		t.Fatalf("unexpected parse error: %#v", err)
	}
	if strings.Contains(err.Error(), "private-response-fragment") {
		t.Fatalf("parse error leaked response content: %v", err)
	}
}

func TestClientPreservesConfiguredBasePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/device/v1/workspace" {
			t.Errorf("request path = %q, want base path prefix", request.URL.Path)
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"data":{"name":"example","status":"active","account_quota":1,"account_quota_unlimited":false}}`)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:           server.URL + "/device",
		APIKeyProvider:    StaticAPIKey("synthetic-test-key"),
		AllowInsecureHTTP: true,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, err := client.GetWorkspace(context.Background(), WithoutRetry()); err != nil {
		t.Fatalf("GetWorkspace() with base path error = %v", err)
	}
}

func TestClientFallsBackToSafeHeaderRequestID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-Id", "req_header_safe")
		// 非法 envelope ID 不应阻断合法 header ID 的诊断回退。
		_, _ = io.WriteString(writer, `{"request_id":"unsafe request id","data":{"name":"example","status":"active","account_quota":1,"account_quota_unlimited":false}}`)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:           server.URL,
		APIKeyProvider:    StaticAPIKey("synthetic-test-key"),
		AllowInsecureHTTP: true,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	result, err := client.GetWorkspace(context.Background(), WithoutRetry())
	if err != nil {
		t.Fatalf("GetWorkspace() error = %v", err)
	}
	if result.RequestID != "req_header_safe" {
		t.Fatalf("RequestID = %q, want safe header fallback", result.RequestID)
	}
}

func TestNilClientReturnsConfigurationError(t *testing.T) {
	var client *Client
	_, err := client.GetWorkspace(context.Background())
	var configurationError *ConfigurationError
	if !errors.As(err, &configurationError) {
		t.Fatalf("expected ConfigurationError, got %v", err)
	}
}
