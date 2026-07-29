package transport

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

const (
	DefaultTimeout          = 30 * time.Second
	DefaultMaxResponseBytes = int64(8 * 1024 * 1024)
	MaximumResponseBytes    = int64(64 * 1024 * 1024)
	DefaultMaxRetries       = 2
	DefaultBaseDelay        = 200 * time.Millisecond
	DefaultMaxDelay         = 5 * time.Second
	MaximumRetries          = 10
)

// APIKeyProvider 每个逻辑调用解析一次 API key，重试会复用同一个值。
type APIKeyProvider func(context.Context) (string, error)

// Doer 抽象 net/http 客户端，主要用于离线测试和受控 transport 注入。
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// RetryConfig 定义仅对 OperationPolicy.Retryable 生效的全局重试预算。
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// Config 是内部 transport 的规范化配置，不依赖公开根包以避免 import cycle。
type Config struct {
	BaseURL          *url.URL
	APIKeyProvider   APIKeyProvider
	Doer             Doer
	Timeout          time.Duration
	MaxResponseBytes int64
	Retry            *RetryConfig
	RandFloat64      func() float64
	Now              func() time.Time
	Sleep            func(context.Context, time.Duration) error
}

// OperationPolicy 由受版本控制的 operation policy 生成，不能根据 HTTP method 推断。
type OperationPolicy struct {
	OperationID string
	Retryable   bool
}

// CallSettings 只允许调用点关闭或收紧全局 timeout/retry。
type CallSettings struct {
	Timeout      time.Duration
	DisableRetry bool
	MaxRetries   *int
}
