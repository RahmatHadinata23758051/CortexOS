package knowledge

import (
	"regexp"
	"strings"
)

var (
	// Expanded patterns for secret detection
	defaultSecretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9\-._~+/]+=*`),
		regexp.MustCompile(`(?i)\b(?:api[_-]?key|secret|token|password|credential|private[_-]?key)\s*[:=]\s*(?:Bearer\s+)?\S+`),
		regexp.MustCompile(`(?i)\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`),
		regexp.MustCompile(`(?i)(?:postgres|mysql|mongodb|redis)://[^\s"']+`),
		regexp.MustCompile(`(?i)-----BEGIN (?:RSA |EC )?PRIVATE KEY-----[^-]+-----END (?:RSA |EC )?PRIVATE KEY-----`),
		regexp.MustCompile(`(?i)\bghp_[A-Za-z0-9_]{36,}\b`),
		regexp.MustCompile(`(?i)\b(?:sk-[A-Za-z0-9]{20,})\b`),
	}
)

// SecretFilter redacts sensitive patterns from content before indexing.
type SecretFilter struct {
	patterns []*regexp.Regexp
	allowed  []*regexp.Regexp
}

// NewSecretFilter creates a filter with default patterns and optional overrides.
func NewSecretFilter(config IngestionConfig) *SecretFilter {
	patterns := append([]*regexp.Regexp(nil), defaultSecretPatterns...)
	for _, p := range config.SecretPatterns {
		if re, err := regexp.Compile(p); err == nil {
			patterns = append(patterns, re)
		}
	}
	var allowed []*regexp.Regexp
	for _, a := range config.AllowedPatterns {
		if re, err := regexp.Compile(a); err == nil {
			allowed = append(allowed, re)
		}
	}
	return &SecretFilter{patterns: patterns, allowed: allowed}
}

// Redact applies secret redaction to content.
func (f *SecretFilter) Redact(content string) string {
	res := content
	for _, p := range f.patterns {
		res = p.ReplaceAllStringFunc(res, func(match string) string {
			for _, allow := range f.allowed {
				if allow.MatchString(match) {
					return match
				}
			}
			lower := strings.ToLower(match)
			if strings.Contains(lower, "bearer") {
				return "Bearer [REDACTED]"
			}
			if strings.Contains(match, "@") && !strings.Contains(lower, ":") {
				return "[EMAIL]"
			}
			if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "mysql://") ||
				strings.HasPrefix(lower, "mongodb://") || strings.HasPrefix(lower, "redis://") {
				return "[CONNECTION_STRING_REDACTED]"
			}
			if strings.Contains(match, "PRIVATE KEY") {
				return "[PRIVATE_KEY_REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return res
}

// RedactContent is a convenience function using default patterns.
func RedactContent(content string) string {
	filter := NewSecretFilter(DefaultIngestionConfig())
	return filter.Redact(content)
}

// IsAllowedPath checks if a relative path is approved for knowledge ingestion.
// Only Markdown files under approved directories are allowed.
func IsAllowedPath(relativePath string) bool {
	if strings.TrimSpace(relativePath) == "" {
		return false
	}
	lower := strings.ToLower(relativePath)
	if !strings.HasSuffix(lower, ".md") {
		return false
	}
	// Deny hidden files/directories
	parts := strings.Split(relativePath, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	// Deny common secret directories
	deniedPrefixes := []string{
		"node_modules/", "vendor/", ".git/", ".github/",
		"credentials/", "secrets/", "keys/", "certs/",
	}
	for _, prefix := range deniedPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	return true
}
