package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client 在生成的 wire client 与 net/http 之间执行统一安全策略。
type Client struct {
	baseURL          *url.URL
	apiKeyProvider   APIKeyProvider
	doer             Doer
	timeout          time.Duration
	maxResponseBytes int64
	retry            RetryConfig
	random           func() float64
	now              func() time.Time
	sleep            func(context.Context, time.Duration) error
}

// New 创建 transport；公开 Config 会先做映射，这里仍保留纵深校验。
func New(config Config) (*Client, error) {
	if config.BaseURL == nil {
		return nil, configurationError("BaseURL", "is required")
	}
	baseURL, err := normalizeInternalBaseURL(config.BaseURL)
	if err != nil {
		return nil, err
	}
	if config.APIKeyProvider == nil {
		return nil, configurationError("APIKeyProvider", "is required")
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	if timeout < 0 {
		return nil, configurationError("Timeout", "must be greater than zero")
	}
	maximumBytes := config.MaxResponseBytes
	if maximumBytes == 0 {
		maximumBytes = DefaultMaxResponseBytes
	}
	if maximumBytes < 0 {
		return nil, configurationError("MaxResponseBytes", "must be greater than zero")
	}
	if maximumBytes > MaximumResponseBytes {
		return nil, configurationError("MaxResponseBytes", "must not exceed 64 MiB")
	}
	retry, err := normalizeRetry(config.Retry)
	if err != nil {
		return nil, err
	}

	doer := normalizeDoer(config.Doer)
	random := config.RandFloat64
	if random == nil {
		random = rand.Float64
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	sleep := config.Sleep
	if sleep == nil {
		sleep = sleepWithContext
	}
	return &Client{
		baseURL:          baseURL,
		apiKeyProvider:   config.APIKeyProvider,
		doer:             doer,
		timeout:          timeout,
		maxResponseBytes: maximumBytes,
		retry:            retry,
		random:           random,
		now:              now,
		sleep:            sleep,
	}, nil
}

func normalizeInternalBaseURL(value *url.URL) (*url.URL, error) {
	copyURL := *value
	if copyURL.Scheme == "" || copyURL.Host == "" || copyURL.Opaque != "" {
		return nil, configurationError("BaseURL", "must be an absolute URL")
	}
	if copyURL.User != nil || copyURL.RawQuery != "" || copyURL.ForceQuery || copyURL.Fragment != "" || copyURL.RawFragment != "" {
		return nil, configurationError("BaseURL", "contains disallowed URL components")
	}
	if copyURL.Scheme != "https" && !(copyURL.Scheme == "http" && isLoopback(copyURL.Hostname())) {
		return nil, configurationError("BaseURL", "must use HTTPS or loopback HTTP")
	}
	copyURL.Path = cleanURLPath(copyURL.Path)
	copyURL.RawPath = ""
	if copyURL.Path == "/" {
		copyURL.Path = ""
	}
	return &copyURL, nil
}

func normalizeRetry(config *RetryConfig) (RetryConfig, error) {
	if config == nil {
		return RetryConfig{
			MaxRetries: DefaultMaxRetries,
			BaseDelay:  DefaultBaseDelay,
			MaxDelay:   DefaultMaxDelay,
		}, nil
	}
	retry := *config
	if retry.MaxRetries < 0 || retry.MaxRetries > MaximumRetries {
		return RetryConfig{}, configurationError("Retry.MaxRetries", "must be between 0 and 10")
	}
	if retry.BaseDelay == 0 {
		retry.BaseDelay = DefaultBaseDelay
	}
	if retry.MaxDelay == 0 {
		retry.MaxDelay = DefaultMaxDelay
	}
	if retry.BaseDelay < 0 {
		return RetryConfig{}, configurationError("Retry.BaseDelay", "must be greater than zero")
	}
	if retry.MaxDelay < 0 {
		return RetryConfig{}, configurationError("Retry.MaxDelay", "must be greater than zero")
	}
	return retry, nil
}

func normalizeDoer(doer Doer) Doer {
	if doer == nil {
		return &http.Client{CheckRedirect: rejectRedirect}
	}
	if client, ok := doer.(*http.Client); ok {
		if client == nil {
			// typed nil 进入 interface 后不再等于 nil，必须显式处理以保留安全默认 client。
			return &http.Client{CheckRedirect: rejectRedirect}
		}
		clone := *client
		// SDK 不执行浏览器式跳转，且不能把 X-Api-Key 带到 redirect target。
		clone.CheckRedirect = rejectRedirect
		clone.Jar = nil
		clone.Timeout = 0
		return &clone
	}
	// 任意 Doer 仅留给 internal 离线测试；公开 Config 只映射受控的 *http.Client。
	return doer
}

func rejectRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

// BaseURL 返回防御性副本，供公开 Client 构造生成请求。
func (c *Client) BaseURL() *url.URL {
	copyURL := *c.baseURL
	return &copyURL
}

// StartOperation 在公开 facade 进入时启动总预算，使 request build 与 response parse 也计入 timeout。
func (c *Client) StartOperation(ctx context.Context, settings CallSettings) (context.Context, context.CancelFunc, time.Duration, error) {
	if ctx == nil {
		return nil, nil, 0, configurationError("context", "must not be nil")
	}
	timeout, err := c.effectiveTimeout(settings)
	if err != nil {
		return nil, nil, 0, err
	}
	operationContext, cancel := context.WithTimeout(ctx, timeout)
	return operationContext, cancel, timeout, nil
}

// OperationContextError 把总预算状态转换为不携带 cause 的公开错误。
func OperationContextError(ctx context.Context, operationID string, timeout time.Duration) error {
	if ctx == nil || ctx.Err() == nil {
		return nil
	}
	return contextError(ctx, safeDiagnostic(operationID, 128), timeout)
}

// Do 执行一个已由 operation policy 分类的逻辑请求。
func (c *Client) Do(ctx context.Context, request *http.Request, policy OperationPolicy, settings CallSettings) (*http.Response, error) {
	if ctx == nil {
		return nil, configurationError("context", "must not be nil")
	}
	if request == nil || request.URL == nil {
		return nil, configurationError("request", "must include an absolute URL")
	}
	operationID := safeDiagnostic(policy.OperationID, 128)
	if !targetInsideBase(request.URL, c.baseURL) {
		// 在读取 credential 前拒绝目标，确保 provider 也不会因恶意 URL 被无谓调用。
		return nil, configurationError("request URL", "is outside configured BaseURL scope")
	}

	timeout, err := c.effectiveTimeout(settings)
	if err != nil {
		return nil, err
	}
	if requestErr := request.Context().Err(); requestErr != nil {
		if errors.Is(requestErr, context.DeadlineExceeded) {
			return nil, &TimeoutError{OperationID: operationID, Timeout: timeout}
		}
		return nil, &CanceledError{OperationID: operationID}
	}
	operationContext, cancel, stopRequestContext := operationContext(ctx, request.Context(), timeout)
	defer cancel()
	defer stopRequestContext()
	if operationContext.Err() != nil {
		// 预算可能已被 request build 消耗完，credential provider 和 HTTP 都不得再执行。
		return nil, contextError(operationContext, operationID, timeout)
	}

	apiKey, err := c.apiKeyProvider(operationContext)
	if err != nil {
		if operationContext.Err() != nil {
			return nil, contextError(operationContext, operationID, timeout)
		}
		// provider 的原始 error 可能包含 secret 或内部存储信息，因此不设置 cause。
		return nil, &NetworkError{OperationID: operationID}
	}
	if operationContext.Err() != nil {
		return nil, contextError(operationContext, operationID, timeout)
	}
	if err := validateAPIKey(apiKey); err != nil {
		return nil, err
	}

	prepared := request.Clone(operationContext)
	prepared.Header = credentialHeaders(prepared.Header, apiKey)
	prepared.Host = ""

	maximumRetries, err := c.effectiveRetries(policy, settings)
	if err != nil {
		return nil, err
	}
	if prepared.Body != nil && prepared.Body != http.NoBody && prepared.GetBody == nil {
		// 无法重放的 body 不可自动重试，否则下一次可能发送空或部分 payload。
		maximumRetries = 0
	}

	for attempt := 0; attempt <= maximumRetries; attempt++ {
		attemptRequest, err := requestForAttempt(prepared, attempt)
		if err != nil {
			return nil, &NetworkError{OperationID: operationID}
		}
		response, requestErr := c.doer.Do(attemptRequest)
		if attemptRequest.Body != nil {
			_ = attemptRequest.Body.Close()
		}

		if requestErr != nil {
			closeResponse(response, c.maxResponseBytes)
			if operationContext.Err() != nil {
				return nil, contextError(operationContext, operationID, timeout)
			}
			if attempt < maximumRetries {
				if err := c.waitBeforeRetry(operationContext, nil, attempt); err != nil {
					return nil, retryWaitError(operationContext, operationID, timeout)
				}
				continue
			}
			if isTimeoutError(requestErr) {
				return nil, &TimeoutError{OperationID: operationID, Timeout: timeout}
			}
			return nil, &NetworkError{OperationID: operationID}
		}
		if response == nil {
			if operationContext.Err() != nil {
				return nil, contextError(operationContext, operationID, timeout)
			}
			if attempt < maximumRetries {
				if err := c.waitBeforeRetry(operationContext, nil, attempt); err != nil {
					return nil, retryWaitError(operationContext, operationID, timeout)
				}
				continue
			}
			return nil, &NetworkError{OperationID: operationID}
		}

		if isRetryableStatus(response.StatusCode) && attempt < maximumRetries {
			header := response.Header.Clone()
			closeResponse(response, c.maxResponseBytes)
			if operationContext.Err() != nil {
				return nil, contextError(operationContext, operationID, timeout)
			}
			if err := c.waitBeforeRetry(operationContext, header, attempt); err != nil {
				return nil, retryWaitError(operationContext, operationID, timeout)
			}
			continue
		}

		buffered, err := bufferResponse(operationContext, response, c.maxResponseBytes, operationID)
		if err != nil {
			if operationContext.Err() != nil {
				return nil, contextError(operationContext, operationID, timeout)
			}
			return nil, err
		}
		return buffered, nil
	}
	return nil, &NetworkError{OperationID: operationID}
}

func operationContext(parent, requestContext context.Context, timeout time.Duration) (context.Context, context.CancelFunc, func() bool) {
	merged, cancelMerged := context.WithCancelCause(parent)
	stop := context.AfterFunc(requestContext, func() {
		cancelMerged(requestContext.Err())
	})
	// parent 若已由 facade 启动总预算，WithTimeout 会继承更早 deadline，不能从 Do 入口重置。
	withTimeout, cancelTimeout := context.WithTimeout(merged, timeout)
	cancel := func() {
		cancelTimeout()
		cancelMerged(context.Canceled)
	}
	return withTimeout, cancel, stop
}

func (c *Client) effectiveTimeout(settings CallSettings) (time.Duration, error) {
	if settings.Timeout < 0 {
		return 0, configurationError("call timeout", "must be greater than zero")
	}
	if settings.Timeout == 0 {
		return c.timeout, nil
	}
	// 调用点只能收紧全局预算，较大的覆盖值不会延长 client timeout。
	return min(settings.Timeout, c.timeout), nil
}

func (c *Client) effectiveRetries(policy OperationPolicy, settings CallSettings) (int, error) {
	if !policy.Retryable || settings.DisableRetry {
		return 0, nil
	}
	if settings.MaxRetries == nil {
		return c.retry.MaxRetries, nil
	}
	if *settings.MaxRetries < 0 {
		return 0, configurationError("call max retries", "must not be negative")
	}
	// 调用点不能给 operation 扩大全局预算，也不能让非 retryable operation 获得重试。
	return min(*settings.MaxRetries, c.retry.MaxRetries), nil
}

func (c *Client) waitBeforeRetry(ctx context.Context, header http.Header, attempt int) error {
	delay := retryDelay(header, attempt, c.retry, c.random, c.now)
	return c.sleep(ctx, delay)
}

func requestForAttempt(request *http.Request, attempt int) (*http.Request, error) {
	clone := request.Clone(request.Context())
	if request.Body == nil || request.Body == http.NoBody {
		clone.Body = request.Body
		return clone, nil
	}
	if attempt == 0 {
		clone.Body = request.Body
		return clone, nil
	}
	body, err := request.GetBody()
	if err != nil {
		return nil, err
	}
	clone.Body = body
	return clone, nil
}

func validateAPIKey(value string) error {
	if strings.TrimSpace(value) == "" {
		return configurationError("APIKeyProvider", "returned an empty API key")
	}
	for _, character := range []byte(value) {
		if character <= 0x1f || character == 0x7f {
			// 控制字符可能改变 header 边界，必须在进入 net/http 前拒绝。
			return configurationError("APIKeyProvider", "returned an invalid header value")
		}
	}
	return nil
}

func credentialHeaders(source http.Header, apiKey string) http.Header {
	headers := make(http.Header, len(source)+1)
	for name, values := range source {
		if strings.EqualFold(name, "Authorization") ||
			strings.EqualFold(name, "Proxy-Authorization") ||
			strings.EqualFold(name, "Cookie") ||
			strings.EqualFold(name, "X-Api-Key") {
			continue
		}
		headers[name] = append([]string(nil), values...)
	}
	// 重建 map 而非 Header.Del，才能移除调用方直接写入的非 canonical 重复 credential key。
	headers.Set("X-Api-Key", apiKey)
	return headers
}

func closeResponse(response *http.Response, maximumBytes int64) {
	if response == nil || response.Body == nil {
		return
	}
	// 放弃的 retry response 也只消费有限字节，避免错误上游借重试制造无界读取。
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maximumBytes+1))
	_ = response.Body.Close()
}

func bufferResponse(ctx context.Context, response *http.Response, maximumBytes int64, operationID string) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		// Deadline 后只关闭而不 drain，避免超时路径继续阻塞在不受控 response body。
		if response.Body != nil {
			_ = response.Body.Close()
		}
		return nil, err
	}
	requestID := safeDiagnostic(response.Header.Get("X-Request-Id"), 128)
	if response.ContentLength > maximumBytes {
		closeResponse(response, maximumBytes)
		return nil, &ResponseParseError{OperationID: operationID, StatusCode: response.StatusCode, RequestID: requestID}
	}
	if response.Body == nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		response.Body = http.NoBody
		response.ContentLength = 0
		return response, nil
	}
	originalBody := response.Body
	body, err := io.ReadAll(io.LimitReader(originalBody, maximumBytes+1))
	_ = originalBody.Close()
	if err != nil {
		return nil, &ResponseParseError{OperationID: operationID, StatusCode: response.StatusCode, RequestID: requestID}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if int64(len(body)) > maximumBytes {
		return nil, &ResponseParseError{OperationID: operationID, StatusCode: response.StatusCode, RequestID: requestID}
	}
	// 生成的 ParseXResponse 仍需读取 Body，因此把受限缓冲区重新装回 response。
	response.Body = io.NopCloser(bytes.NewReader(body))
	response.ContentLength = int64(len(body))
	return response, nil
}

func contextError(ctx context.Context, operationID string, timeout time.Duration) error {
	if errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
		return &TimeoutError{OperationID: operationID, Timeout: timeout}
	}
	return &CanceledError{OperationID: operationID}
}

func retryWaitError(ctx context.Context, operationID string, timeout time.Duration) error {
	if ctx.Err() != nil {
		return contextError(ctx, operationID, timeout)
	}
	// 自定义 sleep 的底层错误不进入公开错误链，避免意外暴露内部调度信息。
	return &NetworkError{OperationID: operationID}
}

func isTimeoutError(err error) bool {
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}

func safeDiagnostic(value string, maximumLength int) string {
	if value == "" || len(value) > maximumLength {
		return ""
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("_.:/-", character)) {
			return ""
		}
	}
	return value
}
