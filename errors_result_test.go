package unifyport

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestAPIErrorRedactsServerPayload(t *testing.T) {
	t.Parallel()
	header := make(http.Header)
	header.Set("X-Request-Id", "req_header")
	body := []byte(`{"request_id":"req_body","error":{"code":"RATE_LIMITED","message":"api-key-secret","details":{"repository":"internal/repo"}}}`)
	err := newAPIError("getWorkspace", http.StatusTooManyRequests, header, body)
	if err.Code != "RATE_LIMITED" || err.RequestID != "req_body" || err.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("unexpected API error: %#v", err)
	}
	if containsAny(err.Error(), "api-key-secret", "internal/repo", string(body)) {
		t.Fatalf("API error leaked response payload: %v", err)
	}
	if len(err.Error()) == 0 {
		t.Fatal("expected stable error message")
	}
}

func TestAPIErrorFiltersDiagnostics(t *testing.T) {
	t.Parallel()
	header := make(http.Header)
	header.Set("X-Request-Id", "header\nsecret")
	err := newAPIError("operation\nsecret", http.StatusBadRequest, header, []byte(`{"error":{"code":"BAD CODE secret"}}`))
	if err.Code != "HTTP_400" || err.RequestID != "" || err.OperationID != "" {
		t.Fatalf("diagnostics were not filtered: %#v", err)
	}
}

func TestResultOnlyExposesAllowlistedHeaders(t *testing.T) {
	t.Parallel()
	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	header.Set("X-Request-Id", "req_123")
	header.Set("X-RateLimit-Remaining", "9")
	header.Set("Set-Cookie", "session=secret")
	header.Set("Server", "internal-hostname")
	header.Set("Authorization", "Bearer secret")
	header.Set("Retry-After", "invalid internal note")
	header.Set("X-RateLimit-Limit", "-1")
	header.Set("X-RateLimit-Reset", "internal-host")
	result := newResult("data", http.StatusOK, "req_123", "client_123", header)
	if result.Data != "data" || result.StatusCode != http.StatusOK || result.RequestID != "req_123" || result.ClientRequestID != "client_123" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Headers.Get("Content-Type") != "application/json" || result.Headers.Get("X-RateLimit-Remaining") != "9" {
		t.Fatal("expected allowlisted headers")
	}
	for _, name := range []string{"Set-Cookie", "Server", "Authorization"} {
		if result.Headers.Get(name) != "" {
			t.Fatalf("unsafe header %s was exposed", name)
		}
	}
	for _, name := range []string{"Retry-After", "X-RateLimit-Limit", "X-RateLimit-Reset"} {
		if result.Headers.Get(name) != "" {
			t.Fatalf("malformed header %s was exposed", name)
		}
	}
	clone := result.Headers.Clone()
	clone.Set("X-Request-Id", "mutated")
	if result.Headers.Get("X-Request-Id") != "req_123" {
		t.Fatal("header snapshot was mutated through Clone")
	}
}

func TestHeaderSnapshotZeroValueAndSemanticNormalization(t *testing.T) {
	t.Parallel()
	var zero HeaderSnapshot
	if zero.Get("X-Request-Id") != "" || zero.Values("X-Request-Id") != nil || len(zero.Clone()) != 0 {
		t.Fatal("zero-value HeaderSnapshot is not safe")
	}
	header := make(http.Header)
	header.Set("Content-Type", `Application/JSON; Charset="utf-8"; boundary="internal-secret"`)
	header.Set("Date", "Sun, 06 Nov 1994 08:49:37 GMT")
	header.Set("Retry-After", "120")
	header.Set("X-Request-Id", "req_123")
	header.Set("X-RateLimit-Remaining", "12")
	snapshot := newHeaderSnapshot(header)
	if snapshot.Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected media type: %q", snapshot.Get("Content-Type"))
	}
	if strings.Contains(snapshot.Get("Content-Type"), "internal-secret") {
		t.Fatal("Content-Type parameter leaked through snapshot")
	}
	if snapshot.Get("Date") != "Sun, 06 Nov 1994 08:49:37 GMT" || snapshot.Get("Retry-After") != "120" {
		t.Fatal("valid timing headers were not retained")
	}
	if snapshot.Get("X-Request-Id") != "req_123" || snapshot.Get("X-RateLimit-Remaining") != "12" {
		t.Fatal("valid diagnostic headers were not retained")
	}
}

func TestPaginationErrorSupportsErrorsIsWithoutCause(t *testing.T) {
	t.Parallel()
	if !errors.Is(&PaginationError{Reason: PaginationCanceled}, contextCanceled()) {
		t.Fatal("expected cancellation compatibility")
	}
	if strings.Contains((&PaginationError{Reason: PaginationRepeatedCursor}).Error(), "cursor-value") {
		t.Fatal("pagination error leaked cursor")
	}
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if candidate != "" && strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func TestAPIErrorNumericCodeIsIndependentAndFiltered(t *testing.T) {
	// 错误类型不能使原 code/request ID 消失，也不能把任意上游诊断字符串暴露出去。
	for _, raw := range []string{`"12345"`, `null`, `12345`, `"01234"`, `"123456"`, `"secret=value"`, `"１２３４５"`} {
		body := []byte(`{"request_id":"req_example","error":{"code":"BAD_REQUEST","numeric_code":` + raw + `}}`)
		result := newAPIError("getWorkspace", 400, nil, body)
		expected := ""
		if raw == `"12345"` {
			expected = "12345"
		}
		if result.NumericCode != expected || result.Code != "BAD_REQUEST" || result.RequestID != "req_example" {
			t.Fatalf("numeric metadata: %#v", result)
		}
	}
}
