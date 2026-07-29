package unifyport

import (
	"time"

	"github.com/Unify-Port/UnifyPort-SDK-GO/internal/transport"
)

// CallOption 只能由本包提供，以防调用方绕过 timeout 或 retry 上限。
type CallOption interface {
	apply(*callSettings) error
}

type callSettings struct {
	timeout      time.Duration
	disableRetry bool
	maxRetries   *int
}

type callOptionFunc func(*callSettings) error

func (option callOptionFunc) apply(settings *callSettings) error {
	return option(settings)
}

// WithTimeout 收紧单次调用的 timeout；大于全局值时仍以全局值为准。
func WithTimeout(timeout time.Duration) CallOption {
	return callOptionFunc(func(settings *callSettings) error {
		if timeout <= 0 {
			return &ConfigurationError{Field: "call timeout", Reason: "must be greater than zero"}
		}
		settings.timeout = timeout
		return nil
	})
}

// WithoutRetry 关闭单次调用的自动 retry。
func WithoutRetry() CallOption {
	return callOptionFunc(func(settings *callSettings) error {
		settings.disableRetry = true
		settings.maxRetries = nil
		return nil
	})
}

// WithMaxRetries 收紧单次调用的 retry 次数，不能扩大 client 或 operation policy 的预算。
func WithMaxRetries(maxRetries int) CallOption {
	return callOptionFunc(func(settings *callSettings) error {
		if maxRetries < 0 {
			return &ConfigurationError{Field: "call max retries", Reason: "must not be negative"}
		}
		value := maxRetries
		settings.maxRetries = &value
		settings.disableRetry = false
		return nil
	})
}

func resolveCallOptions(options []CallOption) (transport.CallSettings, error) {
	settings := callSettings{}
	for _, option := range options {
		if option == nil {
			return transport.CallSettings{}, &ConfigurationError{Field: "CallOption", Reason: "must not be nil"}
		}
		if err := option.apply(&settings); err != nil {
			return transport.CallSettings{}, err
		}
	}
	return transport.CallSettings{
		Timeout:      settings.timeout,
		DisableRetry: settings.disableRetry,
		MaxRetries:   settings.maxRetries,
	}, nil
}
