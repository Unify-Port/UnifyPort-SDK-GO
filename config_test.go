package unifyport

import (
	"context"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/Unify-Port/UnifyPort-SDK-GO/internal/transport"
)

func TestConfigValidatesBaseURLSecurityBoundary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		baseURL   string
		allowHTTP bool
		wantErr   bool
	}{
		{name: "https", baseURL: "https://device.example.com/api"},
		{name: "remote http", baseURL: "http://device.example.com", allowHTTP: true, wantErr: true},
		{name: "loopback requires opt in", baseURL: "http://127.0.0.1:8080", wantErr: true},
		{name: "ipv4 loopback", baseURL: "http://127.1.2.3:8080", allowHTTP: true},
		{name: "ipv6 loopback", baseURL: "http://[::1]:8080", allowHTTP: true},
		{name: "localhost", baseURL: "http://localhost:8080", allowHTTP: true},
		{name: "userinfo", baseURL: "https://user:pass-secret@device.example.com", wantErr: true},
		{name: "query", baseURL: "https://device.example.com?tenant=secret", wantErr: true},
		{name: "empty query", baseURL: "https://device.example.com?", wantErr: true},
		{name: "fragment", baseURL: "https://device.example.com/#internal", wantErr: true},
		{name: "relative", baseURL: "/v1", wantErr: true},
		{name: "invalid", baseURL: "://credential", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := (Config{
				BaseURL:           test.baseURL,
				APIKeyProvider:    StaticAPIKey("test-key"),
				AllowInsecureHTTP: test.allowHTTP,
			}).transportConfig()
			if test.wantErr && err == nil {
				t.Fatal("expected configuration error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err != nil {
				var configurationError *ConfigurationError
				if !errors.As(err, &configurationError) {
					t.Fatalf("expected ConfigurationError, got %T", err)
				}
				if containsAny(err.Error(), "pass-secret", "tenant=secret", "credential") {
					t.Fatalf("configuration error leaked input: %v", err)
				}
			}
		})
	}
}

func mustParseURL(t *testing.T, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	return parsed
}

func TestConfigRequiresAPIKeyProvider(t *testing.T) {
	t.Parallel()
	_, err := (Config{BaseURL: "https://device.example.com"}).transportConfig()
	var configurationError *ConfigurationError
	if !errors.As(err, &configurationError) {
		t.Fatalf("expected ConfigurationError, got %v", err)
	}
}

func TestConfiguredHTTPClientCannotFollowRedirectOrInjectJarCookies(t *testing.T) {
	t.Parallel()
	var redirected atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/start":
			if request.Header.Get("Cookie") != "" {
				t.Errorf("SDK request unexpectedly included cookie: %q", request.Header.Get("Cookie"))
			}
			writer.Header().Set("Location", "/redirected")
			writer.WriteHeader(http.StatusFound)
		case "/redirected":
			redirected.Add(1)
			writer.WriteHeader(http.StatusOK)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	jar.SetCookies(mustParseURL(t, server.URL), []*http.Cookie{{Name: "session", Value: "should-not-leave-client"}})
	configuredClient := server.Client()
	configuredClient.Jar = jar
	configuredClient.CheckRedirect = func(*http.Request, []*http.Request) error { return nil }
	runtime, err := newRuntimeTransport(Config{
		BaseURL:        server.URL,
		APIKeyProvider: StaticAPIKey("test-key"),
		HTTPClient:     configuredClient,
	})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/start", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	response, err := runtime.Do(context.Background(), request, transport.OperationPolicy{OperationID: "redirect", Retryable: false}, transport.CallSettings{})
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusFound {
		t.Fatalf("expected redirect response, got %d", response.StatusCode)
	}
	if redirected.Load() != 0 {
		t.Fatalf("redirect target was called %d times", redirected.Load())
	}
	// clone 后覆盖安全字段，不能悄悄修改调用方复用的 http.Client。
	if configuredClient.Jar == nil || configuredClient.CheckRedirect == nil {
		t.Fatal("configured HTTP client was mutated")
	}
}
