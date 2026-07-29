package transport

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var retryableStatusCodes = map[int]struct{}{
	http.StatusRequestTimeout:     {},
	http.StatusTooManyRequests:    {},
	http.StatusBadGateway:         {},
	http.StatusServiceUnavailable: {},
	http.StatusGatewayTimeout:     {},
}

func isRetryableStatus(statusCode int) bool {
	_, ok := retryableStatusCodes[statusCode]
	return ok
}

func retryDelay(header http.Header, attempt int, retry RetryConfig, random func() float64, now func() time.Time) time.Duration {
	capDelay := exponentialCap(retry.BaseDelay, retry.MaxDelay, attempt)
	randomValue := random()
	if math.IsNaN(randomValue) || randomValue < 0 {
		randomValue = 0
	}
	if randomValue >= 1 {
		randomValue = math.Nextafter(1, 0)
	}
	jitter := time.Duration(randomValue * float64(capDelay))
	retryAfter := parseRetryAfter(header.Get("Retry-After"), now())
	delay := max(jitter, retryAfter)
	return min(delay, retry.MaxDelay)
}

func exponentialCap(baseDelay, maxDelay time.Duration, attempt int) time.Duration {
	if attempt <= 0 {
		return min(baseDelay, maxDelay)
	}
	capDelay := baseDelay
	for range attempt {
		if capDelay >= maxDelay || capDelay > time.Duration(math.MaxInt64)/2 {
			return maxDelay
		}
		capDelay *= 2
	}
	return min(capDelay, maxDelay)
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && !math.IsInf(seconds, 0) {
		maximumSeconds := float64(time.Duration(math.MaxInt64)) / float64(time.Second)
		if seconds >= maximumSeconds {
			return time.Duration(math.MaxInt64)
		}
		return time.Duration(seconds * float64(time.Second))
	}
	when, err := http.ParseTime(value)
	if err != nil || !when.After(now) {
		return 0
	}
	return when.Sub(now)
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer func() {
		// 取消路径停止 timer，避免长 backoff 在调用结束后继续占用资源。
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
