package unifyport

import (
	"context"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/Unify-Port/UnifyPort-SDK-GO/internal/transport"
)

// Client 是 Device API 的公开入口；底层 wire client 保持在 internal，避免绕过统一安全策略。
type Client struct {
	baseURL string
	runtime *transport.Client
}

// NewClient 校验配置并创建可并发使用的 Device API client。
func NewClient(config Config) (*Client, error) {
	runtime, err := newRuntimeTransport(config)
	if err != nil {
		return nil, err
	}
	baseURL := strings.TrimSuffix(runtime.BaseURL().String(), "/") + "/"
	return &Client{
		// wire 使用 relative path 解析；尾斜杠确保配置的 base path 被视为目录而不会被替换。
		baseURL: baseURL,
		runtime: runtime,
	}, nil
}

// operationData 在生成 parser 与公开 Result 之间传递业务 data 和响应 envelope 元数据。
type operationData[T any] struct {
	Data            T
	RequestID       *string
	ClientRequestID *string
}

// executeOperation 让全部生成方法共享同一鉴权、retry、错误净化与结果边界。
func executeOperation[T any](
	ctx context.Context,
	client *Client,
	operation Operation,
	build func(string) (*http.Request, error),
	parse func(*http.Response) (operationData[T], error),
	options ...CallOption,
) (Result[T], error) {
	var zero Result[T]
	if client == nil || client.runtime == nil {
		return zero, &ConfigurationError{Field: "Client", Reason: "must not be nil"}
	}
	if build == nil || parse == nil {
		return zero, &ConfigurationError{Field: "operation", Reason: "is not executable"}
	}
	settings, err := resolveCallOptions(options)
	if err != nil {
		return zero, err
	}
	operationContext, cancelOperation, timeout, err := client.runtime.StartOperation(ctx, settings)
	if err != nil {
		return zero, err
	}
	defer cancelOperation()
	if err := transport.OperationContextError(operationContext, operation.ID, timeout); err != nil {
		return zero, err
	}

	request, buildErr := build(client.baseURL)
	if err := transport.OperationContextError(operationContext, operation.ID, timeout); err != nil {
		// 同步 builder 无法安全中断，但完成后绝不能在超时状态下继续读取 credential 或发送 HTTP。
		return zero, err
	}
	if buildErr != nil {
		// builder 的原始错误可能包含请求值，因此公开边界只返回稳定说明。
		return zero, &ConfigurationError{Field: "operation request", Reason: "could not be encoded"}
	}
	response, err := client.runtime.Do(operationContext, request, transport.OperationPolicy{
		OperationID: operation.ID,
		Retryable:   operation.Retryable,
	}, settings)
	if err != nil {
		return zero, err
	}
	defer func() { _ = response.Body.Close() }()
	if err := transport.OperationContextError(operationContext, operation.ID, timeout); err != nil {
		return zero, err
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(response.Body)
		if err := transport.OperationContextError(operationContext, operation.ID, timeout); err != nil {
			return zero, err
		}
		if readErr != nil {
			return zero, responseParseError(operation.ID, response)
		}
		apiError := newAPIError(operation.ID, response.StatusCode, response.Header, body)
		if err := transport.OperationContextError(operationContext, operation.ID, timeout); err != nil {
			return zero, err
		}
		return zero, apiError
	}

	normalizeParserContentType(response.Header)
	if err := transport.OperationContextError(operationContext, operation.ID, timeout); err != nil {
		return zero, err
	}
	parsed, parseErr := parse(response)
	if err := transport.OperationContextError(operationContext, operation.ID, timeout); err != nil {
		// Parser 同步执行以避免 goroutine 泄漏；超时后即使解析成功也不能返回成功结果。
		return zero, err
	}
	if parseErr != nil {
		// 生成 parser 的 error 可能包含响应片段，不能通过 wrapping 或 cause 暴露。
		return zero, responseParseError(operation.ID, response)
	}
	requestID := safePublicDiagnostic(diagnosticValue(parsed.RequestID), 128)
	if requestID == "" {
		// envelope 值不安全时仍保留经筛选的 header ID，避免丢失可用的诊断关联信息。
		requestID = safePublicDiagnostic(response.Header.Get("X-Request-Id"), 128)
	}
	result := newResult(
		parsed.Data,
		response.StatusCode,
		requestID,
		diagnosticValue(parsed.ClientRequestID),
		response.Header,
	)
	if err := transport.OperationContextError(operationContext, operation.ID, timeout); err != nil {
		return zero, err
	}
	return result, nil
}

func normalizeParserContentType(header http.Header) {
	mediaType, _, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil || mediaType == "" {
		return
	}
	// HTTP media type 大小写不敏感；规范化后避免生成 parser 把合法 JSON success 当成未知响应。
	header.Set("Content-Type", strings.ToLower(mediaType))
}

func responseParseError(operationID string, response *http.Response) *ResponseParseError {
	statusCode := 0
	requestID := ""
	if response != nil {
		statusCode = response.StatusCode
		requestID = safePublicDiagnostic(response.Header.Get("X-Request-Id"), 128)
	}
	return &ResponseParseError{
		OperationID: safePublicDiagnostic(operationID, 128),
		StatusCode:  statusCode,
		RequestID:   requestID,
	}
}

func diagnosticValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
