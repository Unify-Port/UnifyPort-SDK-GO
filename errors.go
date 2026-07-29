package unifyport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Unify-Port/UnifyPort-SDK-GO/internal/transport"
)

// ConfigurationError 是 transport 配置错误的公开别名。
type ConfigurationError = transport.ConfigurationError

// NetworkError 是已脱敏网络错误的公开别名。
type NetworkError = transport.NetworkError

// TimeoutError 是已脱敏 timeout 错误的公开别名。
type TimeoutError = transport.TimeoutError

// CanceledError 是已脱敏 context 取消错误的公开别名。
type CanceledError = transport.CanceledError

// ResponseParseError 是响应读取或解析错误的公开别名。
type ResponseParseError = transport.ResponseParseError

// APIError 只保留稳定 HTTP 元数据；服务端 message、details、body 和 headers 不会公开。
type APIError struct {
	StatusCode  int
	Code        string
	RequestID   string
	OperationID string
}

func (e *APIError) Error() string {
	if e == nil {
		return "UnifyPort API error"
	}
	if e.OperationID == "" {
		return fmt.Sprintf("UnifyPort API returned %s (HTTP %d)", e.Code, e.StatusCode)
	}
	return fmt.Sprintf("UnifyPort API returned %s (HTTP %d, %s)", e.Code, e.StatusCode, e.OperationID)
}

// PaginationErrorReason 是不包含 cursor 值的稳定分页失败分类。
type PaginationErrorReason string

const (
	PaginationInvalidOptions PaginationErrorReason = "invalid_options"
	PaginationMissingCursor  PaginationErrorReason = "missing_next_cursor"
	PaginationRepeatedCursor PaginationErrorReason = "repeated_next_cursor"
	PaginationPageLimit      PaginationErrorReason = "page_limit_exceeded"
	PaginationCanceled       PaginationErrorReason = "canceled"
	PaginationTimedOut       PaginationErrorReason = "timed_out"
)

// PaginationError 表示 cursor 协议或调用 context 使分页无法安全继续。
type PaginationError struct {
	Reason   PaginationErrorReason
	MaxPages int
}

func (e *PaginationError) Error() string {
	if e == nil {
		return "UnifyPort pagination error"
	}
	switch e.Reason {
	case PaginationInvalidOptions:
		return "UnifyPort pagination MaxPages must be greater than zero"
	case PaginationMissingCursor:
		return "UnifyPort pagination response has_more=true without next_cursor"
	case PaginationRepeatedCursor:
		return "UnifyPort pagination stopped after a repeated next_cursor"
	case PaginationPageLimit:
		return fmt.Sprintf("UnifyPort pagination exceeded MaxPages=%d", e.MaxPages)
	case PaginationCanceled:
		return "UnifyPort pagination canceled"
	case PaginationTimedOut:
		return "UnifyPort pagination timed out"
	default:
		return "UnifyPort pagination error"
	}
}

// Is 支持 errors.Is 检查 context 状态，同时不保存调用方自定义的 cancel cause。
func (e *PaginationError) Is(target error) bool {
	if e == nil {
		return false
	}
	return (e.Reason == PaginationCanceled && target == context.Canceled) ||
		(e.Reason == PaginationTimedOut && target == context.DeadlineExceeded)
}

type errorPayload struct {
	Code      string        `json:"code"`
	RequestID string        `json:"request_id"`
	Error     *errorPayload `json:"error"`
}

func newAPIError(operationID string, statusCode int, header http.Header, body []byte) *APIError {
	code := fmt.Sprintf("HTTP_%d", statusCode)
	requestID := safePublicDiagnostic(header.Get("X-Request-Id"), 128)
	var payload errorPayload
	if json.Unmarshal(body, &payload) == nil {
		diagnostic := payload
		if payload.Error != nil {
			diagnostic = *payload.Error
		}
		if candidate := safePublicDiagnostic(diagnostic.Code, 64); candidate != "" {
			code = candidate
		}
		if candidate := safePublicDiagnostic(payload.RequestID, 128); candidate != "" {
			requestID = candidate
		}
	}
	return &APIError{
		StatusCode:  statusCode,
		Code:        code,
		RequestID:   requestID,
		OperationID: safePublicDiagnostic(operationID, 128),
	}
}

func safePublicDiagnostic(value string, maximumLength int) string {
	if value == "" || len(value) > maximumLength {
		return ""
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '_' || character == '.' || character == ':' || character == '/' || character == '-') {
			return ""
		}
	}
	return value
}
