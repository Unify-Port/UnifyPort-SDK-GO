package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (doer doerFunc) Do(request *http.Request) (*http.Response, error) {
	return doer(request)
}

type trackingBody struct {
	reader io.Reader
	closed atomic.Bool
}

func (body *trackingBody) Read(buffer []byte) (int, error) {
	return body.reader.Read(buffer)
}

func (body *trackingBody) Close() error {
	body.closed.Store(true)
	return nil
}

func TestDoResolvesCredentialOnceAndRetriesSafeResponse(t *testing.T) {
	t.Parallel()
	var providerCalls atomic.Int32
	var attempts atomic.Int32
	firstBody := &trackingBody{reader: strings.NewReader("temporary")}
	finalBody := &trackingBody{reader: strings.NewReader(`{"ok":true}`)}
	var delay time.Duration
	client := mustTransport(t, Config{
		BaseURL: mustURL(t, "https://device.example.com/api"),
		APIKeyProvider: func(context.Context) (string, error) {
			providerCalls.Add(1)
			return "trusted-key", nil
		},
		Doer: doerFunc(func(request *http.Request) (*http.Response, error) {
			attempt := attempts.Add(1)
			if request.Header.Get("X-Api-Key") != "trusted-key" {
				t.Errorf("unexpected API key: %q", request.Header.Get("X-Api-Key"))
			}
			credentialHeaderCount := 0
			for name := range request.Header {
				if strings.EqualFold(name, "X-Api-Key") {
					credentialHeaderCount++
				}
			}
			if credentialHeaderCount != 1 {
				t.Errorf("unexpected API key header variants: %#v", request.Header)
			}
			if request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" || request.Header.Get("Proxy-Authorization") != "" {
				t.Error("caller supplied credentials were not removed")
			}
			if request.Host != "" {
				t.Errorf("custom Host was not cleared: %q", request.Host)
			}
			if attempt == 1 {
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Header:     http.Header{"Retry-After": []string{"2"}},
					Body:       firstBody,
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       finalBody,
			}, nil
		}),
		Retry:       &RetryConfig{MaxRetries: 2, BaseDelay: time.Millisecond, MaxDelay: 50 * time.Millisecond},
		RandFloat64: func() float64 { return 0 },
		Sleep: func(_ context.Context, value time.Duration) error {
			delay = value
			return nil
		},
	})
	request := mustRequest(t, http.MethodGet, "https://device.example.com/api/v1/workspace?cursor=secret", nil)
	request.Header.Set("X-Api-Key", "untrusted-key")
	request.Header.Set("Authorization", "Bearer untrusted")
	request.Header.Set("Cookie", "session=untrusted")
	request.Header.Set("Proxy-Authorization", "Basic untrusted")
	request.Header["x-api-key"] = []string{"noncanonical-untrusted-key"}
	request.Header["authorization"] = []string{"Bearer noncanonical-untrusted"}
	request.Header["cookie"] = []string{"session=noncanonical-untrusted"}
	request.Host = "untrusted.example.com"
	response, err := client.Do(context.Background(), request, OperationPolicy{OperationID: "getWorkspace", Retryable: true}, CallSettings{})
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read buffered response: %v", err)
	}
	if string(body) != `{"ok":true}` || response.ContentLength != int64(len(body)) {
		t.Fatalf("unexpected buffered response: %q", body)
	}
	if providerCalls.Load() != 1 || attempts.Load() != 2 {
		t.Fatalf("provider=%d attempts=%d", providerCalls.Load(), attempts.Load())
	}
	if !firstBody.closed.Load() || !finalBody.closed.Load() {
		t.Fatal("original response bodies were not closed")
	}
	if delay != 50*time.Millisecond {
		t.Fatalf("Retry-After was not capped: %s", delay)
	}
}

func TestDoReplaysRequestBodyOnlyWhenGetBodyIsAvailable(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	var received []string
	var receivedMu sync.Mutex
	client := mustTransport(t, Config{
		BaseURL:        mustURL(t, "https://device.example.com"),
		APIKeyProvider: func(context.Context) (string, error) { return "key", nil },
		Doer: doerFunc(func(request *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Errorf("read request body: %v", err)
			}
			receivedMu.Lock()
			received = append(received, string(body))
			receivedMu.Unlock()
			if attempts.Add(1) == 1 {
				return response(http.StatusServiceUnavailable, "temporary"), nil
			}
			return response(http.StatusOK, "ok"), nil
		}),
		Retry: &RetryConfig{MaxRetries: 1},
		Sleep: func(context.Context, time.Duration) error { return nil },
	})
	replayable := mustRequest(t, http.MethodPost, "https://device.example.com/v1/safe-action", bytes.NewBufferString(`{"value":1}`))
	if replayable.GetBody == nil {
		t.Fatal("test request is unexpectedly non-replayable")
	}
	response, err := client.Do(context.Background(), replayable, OperationPolicy{OperationID: "safeAction", Retryable: true}, CallSettings{})
	if err != nil {
		t.Fatalf("execute replayable request: %v", err)
	}
	_ = response.Body.Close()
	if attempts.Load() != 2 || fmt.Sprint(received) != `[{"value":1} {"value":1}]` {
		t.Fatalf("request body was not replayed exactly: attempts=%d bodies=%v", attempts.Load(), received)
	}

	attempts.Store(0)
	nonReplayable := mustRequest(t, http.MethodPost, "https://device.example.com/v1/safe-action", nil)
	nonReplayable.Body = io.NopCloser(strings.NewReader("one-shot"))
	nonReplayable.GetBody = nil
	response, err = client.Do(context.Background(), nonReplayable, OperationPolicy{OperationID: "safeAction", Retryable: true}, CallSettings{})
	if err != nil {
		t.Fatalf("execute non-replayable request: %v", err)
	}
	_ = response.Body.Close()
	if attempts.Load() != 1 {
		t.Fatalf("non-replayable request retried %d times", attempts.Load())
	}
}

func TestDoNeverRetriesUnsafeOrExplicitlyDisabledOperation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		policy   OperationPolicy
		settings CallSettings
	}{
		{name: "unsafe policy", policy: OperationPolicy{OperationID: "write", Retryable: false}},
		{name: "disabled", policy: OperationPolicy{OperationID: "read", Retryable: true}, settings: CallSettings{DisableRetry: true}},
		{name: "zero budget", policy: OperationPolicy{OperationID: "read", Retryable: true}, settings: CallSettings{MaxRetries: intPointer(0)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var attempts atomic.Int32
			client := mustTransport(t, Config{
				BaseURL:        mustURL(t, "https://device.example.com"),
				APIKeyProvider: func(context.Context) (string, error) { return "key", nil },
				Doer: doerFunc(func(*http.Request) (*http.Response, error) {
					attempts.Add(1)
					return response(http.StatusServiceUnavailable, "temporary"), nil
				}),
				Sleep: func(context.Context, time.Duration) error { return nil },
			})
			result, err := client.Do(context.Background(), mustRequest(t, http.MethodGet, "https://device.example.com/v1/test", nil), test.policy, test.settings)
			if err != nil {
				t.Fatalf("execute request: %v", err)
			}
			_ = result.Body.Close()
			if attempts.Load() != 1 {
				t.Fatalf("unexpected attempts: %d", attempts.Load())
			}
		})
	}
}

func TestDoLimitsResponseAndRestoresBufferedBody(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "exact limit", body: "1234"},
		{name: "over limit", body: "12345", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			original := &trackingBody{reader: strings.NewReader(test.body)}
			client := mustTransport(t, Config{
				BaseURL:          mustURL(t, "https://device.example.com"),
				APIKeyProvider:   func(context.Context) (string, error) { return "key", nil },
				MaxResponseBytes: 4,
				Doer: doerFunc(func(*http.Request) (*http.Response, error) {
					header := make(http.Header)
					header.Set("X-Request-Id", "req_limit")
					return &http.Response{StatusCode: http.StatusOK, Header: header, Body: original}, nil
				}),
			})
			result, err := client.Do(context.Background(), mustRequest(t, http.MethodGet, "https://device.example.com/v1/test", nil), OperationPolicy{OperationID: "limit"}, CallSettings{})
			if !original.closed.Load() {
				t.Fatal("original response body was not closed")
			}
			if test.wantErr {
				var parseError *ResponseParseError
				if !errors.As(err, &parseError) || parseError.StatusCode != http.StatusOK || parseError.RequestID != "req_limit" {
					t.Fatalf("unexpected response limit error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("execute request: %v", err)
			}
			defer result.Body.Close()
			body, readErr := io.ReadAll(result.Body)
			if readErr != nil || string(body) != test.body {
				t.Fatalf("buffered body=%q err=%v", body, readErr)
			}
		})
	}
}

func TestDoRedactsProviderAndNetworkErrors(t *testing.T) {
	t.Parallel()
	providerClient := mustTransport(t, Config{
		BaseURL: mustURL(t, "https://device.example.com"),
		APIKeyProvider: func(context.Context) (string, error) {
			return "", errors.New("vault path and api-key-secret")
		},
		Doer: doerFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("HTTP should not run after provider failure")
			return nil, nil
		}),
	})
	_, err := providerClient.Do(context.Background(), mustRequest(t, http.MethodGet, "https://device.example.com/v1/test", nil), OperationPolicy{OperationID: "provider"}, CallSettings{})
	var networkError *NetworkError
	if !errors.As(err, &networkError) || strings.Contains(err.Error(), "vault") || strings.Contains(err.Error(), "api-key-secret") {
		t.Fatalf("provider failure was not redacted: %v", err)
	}

	networkClient := mustTransport(t, Config{
		BaseURL:        mustURL(t, "https://device.example.com"),
		APIKeyProvider: func(context.Context) (string, error) { return "key", nil },
		Retry:          &RetryConfig{MaxRetries: 0},
		Doer: doerFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial failure exposed api-key-secret")
		}),
	})
	_, err = networkClient.Do(context.Background(), mustRequest(t, http.MethodGet, "https://device.example.com/v1/test?cursor=secret", nil), OperationPolicy{OperationID: "network", Retryable: true}, CallSettings{})
	if !errors.As(err, &networkError) {
		t.Fatalf("expected NetworkError, got %v", err)
	}
	if containsAnyForTransport(err.Error(), "device.example.com", "/v1/test", "cursor", "secret", "dial failure") {
		t.Fatalf("network error leaked detail: %#v %v", networkError, err)
	}
}

func TestDoRejectsCredentialTargetsOutsideScopeBeforeProvider(t *testing.T) {
	t.Parallel()
	var providerCalls atomic.Int32
	client := mustTransport(t, Config{
		BaseURL: mustURL(t, "https://device.example.com/api"),
		APIKeyProvider: func(context.Context) (string, error) {
			providerCalls.Add(1)
			return "key", nil
		},
		Doer: doerFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("out-of-scope request reached HTTP")
			return nil, nil
		}),
	})
	for _, target := range []string{
		"https://evil.example.com/api/v1/test",
		"https://device.example.com/v1/test",
		"https://device.example.com/apievil/v1/test",
		"https://device.example.com/api/%2e%2e/private",
	} {
		_, err := client.Do(context.Background(), mustRequest(t, http.MethodGet, target, nil), OperationPolicy{OperationID: "scope"}, CallSettings{})
		var configurationError *ConfigurationError
		if !errors.As(err, &configurationError) {
			t.Fatalf("target %s: expected ConfigurationError, got %v", target, err)
		}
	}
	if providerCalls.Load() != 0 {
		t.Fatalf("provider called %d times for rejected targets", providerCalls.Load())
	}
}

func TestDoHonorsCancellationDuringRetryBackoff(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	client := mustTransport(t, Config{
		BaseURL:        mustURL(t, "https://device.example.com"),
		APIKeyProvider: func(context.Context) (string, error) { return "key", nil },
		Doer: doerFunc(func(*http.Request) (*http.Response, error) {
			return response(http.StatusTooManyRequests, "wait"), nil
		}),
		Retry: &RetryConfig{MaxRetries: 2},
		Sleep: func(ctx context.Context, _ time.Duration) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := client.Do(ctx, mustRequest(t, http.MethodGet, "https://device.example.com/v1/test", nil), OperationPolicy{OperationID: "cancel", Retryable: true}, CallSettings{})
		result <- err
	}()
	<-started
	cancel()
	err := <-result
	var canceledError *CanceledError
	if !errors.As(err, &canceledError) || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected CanceledError, got %v", err)
	}
}

func TestDoClassifiesTimeoutWithoutRawCause(t *testing.T) {
	t.Parallel()
	client := mustTransport(t, Config{
		BaseURL:        mustURL(t, "https://device.example.com"),
		APIKeyProvider: func(context.Context) (string, error) { return "key", nil },
		Timeout:        20 * time.Millisecond,
		Doer: doerFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, fmt.Errorf("raw timeout cause: %w", request.Context().Err())
		}),
	})
	_, err := client.Do(context.Background(), mustRequest(t, http.MethodGet, "https://device.example.com/v1/test", nil), OperationPolicy{OperationID: "timeout"}, CallSettings{})
	var timeoutError *TimeoutError
	if !errors.As(err, &timeoutError) || !errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "raw timeout cause") {
		t.Fatalf("expected sanitized TimeoutError, got %v", err)
	}
}

func TestDoUsesStartedOperationDeadlineAndSkipsExpiredBudget(t *testing.T) {
	t.Parallel()
	var expectedDeadline time.Time
	var providerCalls atomic.Int32
	var httpCalls atomic.Int32
	client := mustTransport(t, Config{
		BaseURL: mustURL(t, "https://device.example.com"),
		APIKeyProvider: func(ctx context.Context) (string, error) {
			providerCalls.Add(1)
			deadline, ok := ctx.Deadline()
			if !ok || !deadline.Equal(expectedDeadline) {
				t.Errorf("provider deadline = %v, want %v", deadline, expectedDeadline)
			}
			return "synthetic-test-key", nil
		},
		Doer: doerFunc(func(request *http.Request) (*http.Response, error) {
			httpCalls.Add(1)
			deadline, ok := request.Context().Deadline()
			if !ok || !deadline.Equal(expectedDeadline) {
				t.Errorf("HTTP deadline = %v, want %v", deadline, expectedDeadline)
			}
			return response(http.StatusOK, `{}`), nil
		}),
		Timeout: time.Second,
	})

	started, cancel, timeout, err := client.StartOperation(context.Background(), CallSettings{})
	if err != nil {
		t.Fatalf("StartOperation() error = %v", err)
	}
	defer cancel()
	expectedDeadline, _ = started.Deadline()
	result, err := client.Do(started, mustRequest(t, http.MethodGet, "https://device.example.com/v1/test", nil), OperationPolicy{OperationID: "budget"}, CallSettings{})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	_ = result.Body.Close()
	if providerCalls.Load() != 1 || httpCalls.Load() != 1 || timeout != time.Second {
		t.Fatalf("unexpected started-budget state: provider=%d http=%d timeout=%s", providerCalls.Load(), httpCalls.Load(), timeout)
	}

	expired, expiredCancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expiredCancel()
	startedExpired, cancelExpired, expiredTimeout, err := client.StartOperation(expired, CallSettings{})
	if err != nil {
		t.Fatalf("StartOperation(expired) error = %v", err)
	}
	defer cancelExpired()
	_, err = client.Do(startedExpired, mustRequest(t, http.MethodGet, "https://device.example.com/v1/test", nil), OperationPolicy{OperationID: "expired"}, CallSettings{})
	var timeoutError *TimeoutError
	if !errors.As(err, &timeoutError) || !errors.Is(err, context.DeadlineExceeded) || expiredTimeout != time.Second {
		t.Fatalf("Do(expired) error = %v, want TimeoutError", err)
	}
	if providerCalls.Load() != 1 || httpCalls.Load() != 1 {
		t.Fatalf("expired budget invoked provider/HTTP: provider=%d http=%d", providerCalls.Load(), httpCalls.Load())
	}
}

func TestDoRejectsInvalidAPIKeyBeforeHTTP(t *testing.T) {
	t.Parallel()
	for _, apiKey := range []string{"", "   ", "key\nsecret", "key\x00secret"} {
		client := mustTransport(t, Config{
			BaseURL:        mustURL(t, "https://device.example.com"),
			APIKeyProvider: func(context.Context) (string, error) { return apiKey, nil },
			Doer: doerFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("invalid API key reached HTTP")
				return nil, nil
			}),
		})
		_, err := client.Do(context.Background(), mustRequest(t, http.MethodGet, "https://device.example.com/v1/test", nil), OperationPolicy{OperationID: "credential"}, CallSettings{})
		var configurationError *ConfigurationError
		if !errors.As(err, &configurationError) || strings.Contains(err.Error(), apiKey) && apiKey != "" {
			t.Fatalf("unexpected key error for %q: %v", apiKey, err)
		}
	}
}

func TestRetryStatusAndRetryAfterPolicy(t *testing.T) {
	t.Parallel()
	for _, statusCode := range []int{408, 429, 502, 503, 504} {
		if !isRetryableStatus(statusCode) {
			t.Fatalf("status %d should be retryable", statusCode)
		}
	}
	for _, statusCode := range []int{400, 401, 409, 500, 505} {
		if isRetryableStatus(statusCode) {
			t.Fatalf("status %d should not be retryable", statusCode)
		}
	}
	now := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	if got := parseRetryAfter("0.125", now); got != 125*time.Millisecond {
		t.Fatalf("unexpected numeric Retry-After: %s", got)
	}
	if got := parseRetryAfter(now.Add(3*time.Second).Format(http.TimeFormat), now); got != 3*time.Second {
		t.Fatalf("unexpected date Retry-After: %s", got)
	}
	if got := parseRetryAfter("invalid", now); got != 0 {
		t.Fatalf("invalid Retry-After produced %s", got)
	}
}

func TestNewAppliesDefaultsAndValidatesLimits(t *testing.T) {
	t.Parallel()
	client := mustTransport(t, Config{
		BaseURL:        mustURL(t, "https://device.example.com"),
		APIKeyProvider: func(context.Context) (string, error) { return "key", nil },
	})
	if client.timeout != DefaultTimeout || client.maxResponseBytes != DefaultMaxResponseBytes || client.retry.MaxRetries != DefaultMaxRetries {
		t.Fatalf("defaults were not applied: %#v", client)
	}
	for _, config := range []Config{
		{BaseURL: mustURL(t, "https://device.example.com"), APIKeyProvider: func(context.Context) (string, error) { return "key", nil }, Timeout: -time.Second},
		{BaseURL: mustURL(t, "https://device.example.com"), APIKeyProvider: func(context.Context) (string, error) { return "key", nil }, MaxResponseBytes: MaximumResponseBytes + 1},
		{BaseURL: mustURL(t, "https://device.example.com"), APIKeyProvider: func(context.Context) (string, error) { return "key", nil }, Retry: &RetryConfig{MaxRetries: MaximumRetries + 1}},
	} {
		if _, err := New(config); err == nil {
			t.Fatalf("expected invalid config error: %#v", config)
		}
	}
}

func mustTransport(t *testing.T, config Config) *Client {
	t.Helper()
	client, err := New(config)
	if err != nil {
		t.Fatalf("create transport: %v", err)
	}
	return client
}

func mustURL(t *testing.T, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	return parsed
}

func mustRequest(t *testing.T, method, target string, body io.Reader) *http.Request {
	t.Helper()
	request, err := http.NewRequest(method, target, body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	return request
}

func response(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func intPointer(value int) *int {
	return &value
}

func containsAnyForTransport(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if candidate != "" && strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
