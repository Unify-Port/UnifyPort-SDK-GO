package unifyport

import (
	"mime"
	"net/http"
	"regexp"
	"strings"
)

// NoContent 明确表示成功的 204 response 没有业务 payload。
type NoContent struct{}

// Result 包含业务 data 与经过筛选的 HTTP 元数据，不公开 raw response 或 raw body。
type Result[T any] struct {
	Data            T
	StatusCode      int
	RequestID       string
	ClientRequestID string
	Headers         HeaderSnapshot
}

// HeaderSnapshot 只保存明确 allowlist 中的诊断与限流 headers。
type HeaderSnapshot struct {
	values http.Header
}

var safeResponseHeaders = map[string]struct{}{
	"Content-Type":          {},
	"Date":                  {},
	"Retry-After":           {},
	"X-Request-Id":          {},
	"X-Ratelimit-Limit":     {},
	"X-Ratelimit-Remaining": {},
	"X-Ratelimit-Reset":     {},
	"Ratelimit-Limit":       {},
	"Ratelimit-Remaining":   {},
	"Ratelimit-Reset":       {},
}

var nonNegativeDecimalHeader = regexp.MustCompile(`^[0-9]{1,20}$`)

// Get 返回 allowlist header 的首个值。
func (snapshot HeaderSnapshot) Get(name string) string {
	return snapshot.values.Get(name)
}

// Values 返回独立副本，避免调用方修改 Result 内的 header snapshot。
func (snapshot HeaderSnapshot) Values(name string) []string {
	return append([]string(nil), snapshot.values.Values(name)...)
}

// Clone 返回只含 allowlist headers 的可修改副本。
func (snapshot HeaderSnapshot) Clone() http.Header {
	return snapshot.values.Clone()
}

func newResult[T any](data T, statusCode int, requestID, clientRequestID string, header http.Header) Result[T] {
	return Result[T]{
		Data:            data,
		StatusCode:      statusCode,
		RequestID:       safePublicDiagnostic(requestID, 128),
		ClientRequestID: safePublicDiagnostic(clientRequestID, 128),
		Headers:         newHeaderSnapshot(header),
	}
}

func newHeaderSnapshot(header http.Header) HeaderSnapshot {
	values := make(http.Header)
	for name := range safeResponseHeaders {
		for _, value := range header.Values(name) {
			if sanitized, ok := sanitizeResponseHeader(name, value); ok {
				values.Add(name, sanitized)
				// 这些 headers 都是单值语义，只保留首个合法值以限制异常上游放大快照。
				break
			}
		}
	}
	return HeaderSnapshot{values: values}
}

func sanitizeResponseHeader(name, value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 1024 || strings.ContainsAny(value, "\r\n") {
		return "", false
	}
	switch http.CanonicalHeaderKey(name) {
	case "X-Request-Id":
		value = safePublicDiagnostic(value, 128)
		return value, value != ""
	case "X-Ratelimit-Limit", "X-Ratelimit-Remaining", "X-Ratelimit-Reset",
		"Ratelimit-Limit", "Ratelimit-Remaining", "Ratelimit-Reset":
		return value, nonNegativeDecimalHeader.MatchString(value)
	case "Date":
		parsed, err := http.ParseTime(value)
		if err != nil {
			return "", false
		}
		return parsed.UTC().Format(http.TimeFormat), true
	case "Retry-After":
		if nonNegativeDecimalHeader.MatchString(value) {
			return value, true
		}
		parsed, err := http.ParseTime(value)
		if err != nil {
			return "", false
		}
		return parsed.UTC().Format(http.TimeFormat), true
	case "Content-Type":
		mediaType, _, err := mime.ParseMediaType(value)
		if err != nil || len(mediaType) > 127 {
			return "", false
		}
		// 参数可能携带 boundary 或内部标识，公开快照只保留规范化 media type。
		normalized := strings.ToLower(mediaType)
		return normalized, normalized != ""
	default:
		return "", false
	}
}
