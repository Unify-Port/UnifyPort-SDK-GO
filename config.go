package unifyport

import (
	"context"
	"net/http"
	"time"

	"github.com/Unify-Port/UnifyPort-SDK-GO/internal/transport"
)

const (
	// DefaultTimeout 是一个完整逻辑调用（包含 credential、retry 和响应读取）的默认上限。
	DefaultTimeout = transport.DefaultTimeout
	// DefaultMaxResponseBytes 在 JSON 解析前限制异常上游占用的内存。
	DefaultMaxResponseBytes = transport.DefaultMaxResponseBytes
	// MaximumResponseBytes 是公开配置不可突破的硬上限。
	MaximumResponseBytes = transport.MaximumResponseBytes
	// DefaultMaxRetries 表示首次请求之后最多再尝试的次数。
	DefaultMaxRetries = transport.DefaultMaxRetries
)

// APIKeyProvider 在每个逻辑 operation 开始时解析一次 API key。
type APIKeyProvider func(context.Context) (string, error)

// StaticAPIKey 把静态 API key 封装在 provider 闭包中，避免 Client 暴露明文 credential 字段。
func StaticAPIKey(apiKey string) APIKeyProvider {
	return func(context.Context) (string, error) {
		return apiKey, nil
	}
}

// RetryConfig 定义全局 retry 预算；只有生成 policy 明确允许的 operation 才会使用它。
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// Config 描述 Device API Client 的安全运行参数。
type Config struct {
	BaseURL           string
	APIKeyProvider    APIKeyProvider
	HTTPClient        *http.Client
	Timeout           time.Duration
	MaxResponseBytes  int64
	Retry             *RetryConfig
	AllowInsecureHTTP bool
}

func (config Config) transportConfig() (transport.Config, error) {
	baseURL, err := transport.NormalizeBaseURL(config.BaseURL, config.AllowInsecureHTTP)
	if err != nil {
		return transport.Config{}, err
	}
	if config.APIKeyProvider == nil {
		return transport.Config{}, &ConfigurationError{Field: "APIKeyProvider", Reason: "is required"}
	}

	var retry *transport.RetryConfig
	if config.Retry != nil {
		retry = &transport.RetryConfig{
			MaxRetries: config.Retry.MaxRetries,
			BaseDelay:  config.Retry.BaseDelay,
			MaxDelay:   config.Retry.MaxDelay,
		}
	}
	return transport.Config{
		BaseURL:          baseURL,
		APIKeyProvider:   transport.APIKeyProvider(config.APIKeyProvider),
		Doer:             config.HTTPClient,
		Timeout:          config.Timeout,
		MaxResponseBytes: config.MaxResponseBytes,
		Retry:            retry,
	}, nil
}

func newRuntimeTransport(config Config) (*transport.Client, error) {
	transportConfig, err := config.transportConfig()
	if err != nil {
		return nil, err
	}
	return transport.New(transportConfig)
}
