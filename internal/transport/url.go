package transport

import (
	"net"
	"net/url"
	"path"
	"strings"
)

// NormalizeBaseURL 校验并规范化公开 API 根地址；远端明文 HTTP 永远不会被放行。
func NormalizeBaseURL(raw string, allowInsecureHTTP bool) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Opaque != "" {
		// 原始解析错误可能回显完整输入，因此只返回稳定的配置说明。
		return nil, configurationError("BaseURL", "must be an absolute URL")
	}
	if parsed.User != nil {
		return nil, configurationError("BaseURL", "must not contain username or password")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery {
		return nil, configurationError("BaseURL", "must not contain query")
	}
	if parsed.Fragment != "" || parsed.RawFragment != "" {
		return nil, configurationError("BaseURL", "must not contain fragment")
	}
	if parsed.Scheme != "https" {
		allowedLoopback := parsed.Scheme == "http" && allowInsecureHTTP && isLoopback(parsed.Hostname())
		if !allowedLoopback {
			return nil, configurationError("BaseURL", "must use HTTPS; loopback HTTP requires AllowInsecureHTTP")
		}
	}

	// 清理 dot segment，避免 credential scope 与上游实际路由看到不同路径。
	parsed.Path = cleanURLPath(parsed.Path)
	parsed.RawPath = ""
	if parsed.Path == "/" {
		parsed.Path = ""
	}
	return parsed, nil
}

func configurationError(field, reason string) error {
	return &ConfigurationError{Field: field, Reason: reason}
}

func isLoopback(hostname string) bool {
	normalized := strings.TrimSuffix(strings.ToLower(hostname), ".")
	if normalized == "localhost" {
		return true
	}
	ip := net.ParseIP(normalized)
	return ip != nil && ip.IsLoopback()
}

func cleanURLPath(value string) string {
	if value == "" {
		return "/"
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(value, "/"))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func targetInsideBase(target, base *url.URL) bool {
	if target == nil || base == nil || target.User != nil {
		return false
	}
	if !sameOrigin(target, base) {
		return false
	}
	basePath := strings.TrimSuffix(cleanURLPath(base.Path), "/")
	targetPath := cleanURLPath(target.Path)
	return basePath == "" || basePath == "/" || targetPath == basePath || strings.HasPrefix(targetPath, basePath+"/")
}

func sameOrigin(left, right *url.URL) bool {
	return strings.EqualFold(left.Scheme, right.Scheme) &&
		strings.EqualFold(left.Hostname(), right.Hostname()) &&
		effectivePort(left) == effectivePort(right)
}

func effectivePort(value *url.URL) string {
	if port := value.Port(); port != "" {
		return port
	}
	switch strings.ToLower(value.Scheme) {
	case "https":
		return "443"
	case "http":
		return "80"
	default:
		return ""
	}
}
