package transport

import (
	"context"
	"fmt"
	"time"
)

// ConfigurationError 表示 SDK 配置违反了公开的安全边界。
type ConfigurationError struct {
	Field  string
	Reason string
}

func (e *ConfigurationError) Error() string {
	if e == nil {
		return "UnifyPort SDK configuration error"
	}
	if e.Field == "" {
		return "UnifyPort SDK configuration error: " + e.Reason
	}
	return fmt.Sprintf("UnifyPort SDK configuration error: %s %s", e.Field, e.Reason)
}

// NetworkError 表示请求未获得可解析的 HTTP 响应；底层 cause 不公开，避免泄漏 credential 或响应片段。
type NetworkError struct {
	OperationID string
}

func (e *NetworkError) Error() string {
	return operationMessage("UnifyPort network request failed", e.OperationID)
}

// TimeoutError 表示调用在配置或调用方 deadline 内未完成。
type TimeoutError struct {
	OperationID string
	Timeout     time.Duration
}

func (e *TimeoutError) Error() string {
	return operationMessage("UnifyPort request timed out", e.OperationID)
}

// Is 保留 errors.Is 的惯用语义，但不通过 Unwrap 暴露底层错误。
func (e *TimeoutError) Is(target error) bool {
	return target == context.DeadlineExceeded
}

// CanceledError 表示调用被 context 主动取消。
type CanceledError struct {
	OperationID string
}

func (e *CanceledError) Error() string {
	return operationMessage("UnifyPort request canceled", e.OperationID)
}

// Is 保留 errors.Is 的惯用语义，但不携带可能含敏感信息的取消 cause。
func (e *CanceledError) Is(target error) bool {
	return target == context.Canceled
}

// ResponseParseError 表示响应体读取失败、超限或无法交给生成代码解析。
type ResponseParseError struct {
	OperationID string
	StatusCode  int
	RequestID   string
}

func (e *ResponseParseError) Error() string {
	return operationMessage("unable to read UnifyPort API response", e.OperationID)
}

func operationMessage(message, operationID string) string {
	if operationID == "" {
		return message
	}
	return fmt.Sprintf("%s (%s)", message, operationID)
}
