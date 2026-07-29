package unifyport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type timeoutRoundTripper func(*http.Request) (*http.Response, error)

func (roundTripper timeoutRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTripper(request)
}

func TestExecuteOperationTimeoutIncludesRequestBuild(t *testing.T) {
	var providerCalls atomic.Int32
	var httpCalls atomic.Int32
	var parseCalls atomic.Int32
	client, err := NewClient(Config{
		BaseURL: "https://device.example.com",
		APIKeyProvider: func(context.Context) (string, error) {
			providerCalls.Add(1)
			return "synthetic-test-key", nil
		},
		HTTPClient: &http.Client{Transport: timeoutRoundTripper(func(*http.Request) (*http.Response, error) {
			httpCalls.Add(1)
			return nil, errors.New("HTTP must not run after build consumed timeout")
		})},
		Timeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	result, err := executeOperation(
		context.Background(),
		client,
		Operation{ID: "slowBuild"},
		func(baseURL string) (*http.Request, error) {
			// 同步 marshal/build 不启动 goroutine；等待足够久以验证它消耗同一个总预算。
			time.Sleep(30 * time.Millisecond)
			return http.NewRequest(http.MethodGet, baseURL+"v1/workspace", nil)
		},
		func(*http.Response) (operationData[string], error) {
			parseCalls.Add(1)
			return operationData[string]{Data: "unexpected"}, nil
		},
	)
	var timeoutError *TimeoutError
	if !errors.As(err, &timeoutError) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("executeOperation() error = %v, want TimeoutError", err)
	}
	if providerCalls.Load() != 0 || httpCalls.Load() != 0 || parseCalls.Load() != 0 {
		t.Fatalf("expired build continued: provider=%d http=%d parse=%d", providerCalls.Load(), httpCalls.Load(), parseCalls.Load())
	}
	if result.Data != "" || result.StatusCode != 0 || result.RequestID != "" || result.ClientRequestID != "" || len(result.Headers.Clone()) != 0 {
		t.Fatalf("expired build returned result: %#v", result)
	}
}

func TestExecuteOperationTimeoutRejectsSuccessfulSlowParse(t *testing.T) {
	var providerCalls atomic.Int32
	var httpCalls atomic.Int32
	var parseCalls atomic.Int32
	client, err := NewClient(Config{
		BaseURL: "https://device.example.com",
		APIKeyProvider: func(context.Context) (string, error) {
			providerCalls.Add(1)
			return "synthetic-test-key", nil
		},
		HTTPClient: &http.Client{Transport: timeoutRoundTripper(func(*http.Request) (*http.Response, error) {
			httpCalls.Add(1)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"data":"value"}`)),
			}, nil
		})},
		Timeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	result, err := executeOperation(
		context.Background(),
		client,
		Operation{ID: "slowParse"},
		func(baseURL string) (*http.Request, error) {
			return http.NewRequest(http.MethodGet, baseURL+"v1/workspace", nil)
		},
		func(*http.Response) (operationData[string], error) {
			parseCalls.Add(1)
			// Parser 保持同步；即使最终返回 data，超过总预算后也必须丢弃成功结果。
			time.Sleep(200 * time.Millisecond)
			return operationData[string]{Data: "must-not-succeed"}, nil
		},
	)
	var timeoutError *TimeoutError
	if !errors.As(err, &timeoutError) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("executeOperation() error = %v, want TimeoutError", err)
	}
	if providerCalls.Load() != 1 || httpCalls.Load() != 1 || parseCalls.Load() != 1 {
		t.Fatalf("unexpected call counts: provider=%d http=%d parse=%d", providerCalls.Load(), httpCalls.Load(), parseCalls.Load())
	}
	if result.Data != "" || result.StatusCode != 0 || result.RequestID != "" || result.ClientRequestID != "" || len(result.Headers.Clone()) != 0 {
		t.Fatalf("slow parser returned success: %#v", result)
	}
}
