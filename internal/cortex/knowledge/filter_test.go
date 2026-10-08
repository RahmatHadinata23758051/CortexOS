package knowledge

import (
	"strings"
	"testing"
)

func TestSecretFilterEmail(t *testing.T) {
	filter := NewSecretFilter(DefaultIngestionConfig())
	input := "Contact user@example.com for access."
	output := filter.Redact(input)
	if strings.Contains(output, "user@example.com") {
		t.Errorf("email not redacted: %q", output)
	}
	if !strings.Contains(output, "[EMAIL]") {
		t.Errorf("email redaction marker missing: %q", output)
	}
}

func TestSecretFilterAPIKey(t *testing.T) {
	filter := NewSecretFilter(DefaultIngestionConfig())
	input := "api_key=abc123secret456"
	output := filter.Redact(input)
	if strings.Contains(output, "abc123secret456") {
		t.Errorf("api key not redacted: %q", output)
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Errorf("redaction marker missing: %q", output)
	}
}

func TestSecretFilterBearerToken(t *testing.T) {
	filter := NewSecretFilter(DefaultIngestionConfig())
	input := "Authorization: Bearer token_xyz_123"
	output := filter.Redact(input)
	if strings.Contains(output, "token_xyz_123") {
		t.Errorf("bearer token not redacted: %q", output)
	}
	if !strings.Contains(output, "Bearer [REDACTED]") {
		t.Errorf("bearer redaction marker missing: %q", output)
	}
}

func TestSecretFilterPassword(t *testing.T) {
	filter := NewSecretFilter(DefaultIngestionConfig())
	input := "password=mysecretpassword123"
	output := filter.Redact(input)
	if strings.Contains(output, "mysecretpassword123") {
		t.Errorf("password not redacted: %q", output)
	}
}

func TestSecretFilterConnectionString(t *testing.T) {
	filter := NewSecretFilter(DefaultIngestionConfig())
	input := "DATABASE_URL=postgres://user:pass@localhost/db"
	output := filter.Redact(input)
	if strings.Contains(output, "postgres://user:pass@localhost/db") {
		t.Errorf("connection string not redacted: %q", output)
	}
	if !strings.Contains(output, "[CONNECTION_STRING_REDACTED]") {
		t.Errorf("connection string marker missing: %q", output)
	}
}

func TestSecretFilterPrivateKey(t *testing.T) {
	filter := NewSecretFilter(DefaultIngestionConfig())
	input := "-----BEGIN PRIVATE KEY-----\nMIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC7VJTUt9Us8cKB\n-----END PRIVATE KEY-----"
	output := filter.Redact(input)
	if strings.Contains(output, "MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC7VJTUt9Us8cKB") {
		t.Errorf("private key not redacted: %q", output)
	}
	if !strings.Contains(output, "[PRIVATE_KEY_REDACTED]") {
		t.Errorf("private key marker missing: %q", output)
	}
}

func TestSecretFilterGitHubToken(t *testing.T) {
	filter := NewSecretFilter(DefaultIngestionConfig())
	input := "ghp_abcdefghijklmnopqrstuvwxyz1234567890"
	output := filter.Redact(input)
	if strings.Contains(output, "ghp_abcdefghijklmnopqrstuvwxyz1234567890") {
		t.Errorf("GitHub token not redacted: %q", output)
	}
}

func TestSecretFilterAllowlist(t *testing.T) {
	config := IngestionConfig{
		AllowedPatterns: []string{`(?i)example\.com`},
	}
	filter := NewSecretFilter(config)
	input := "user@example.com is our test email"
	output := filter.Redact(input)
	if !strings.Contains(output, "user@example.com") {
		t.Errorf("allowed email was redacted: %q", output)
	}
}

func TestSecretFilterDeterministic(t *testing.T) {
	filter := NewSecretFilter(DefaultIngestionConfig())
	input := "api_key=secret123 and token=abc456"
	out1 := filter.Redact(input)
	out2 := filter.Redact(input)
	if out1 != out2 {
		t.Errorf("redaction not deterministic: %q != %q", out1, out2)
	}
}

func TestSecretFilterNoFalsePositiveOnCode(t *testing.T) {
	filter := NewSecretFilter(DefaultIngestionConfig())
	input := "func getToken() string { return \"abc\" }"
	output := filter.Redact(input)
	if strings.Contains(output, "[REDACTED]") {
		t.Errorf("code falsely redacted: %q", output)
	}
}

func TestIsAllowedPathMarkdown(t *testing.T) {
	allowed := []string{"notes/architecture.md", "docs/specs.md", "README.md", "a/b/c/d.md"}
	for _, path := range allowed {
		if !IsAllowedPath(path) {
			t.Errorf("expected allowed: %s", path)
		}
	}
}

func TestIsAllowedPathNonMarkdown(t *testing.T) {
	denied := []string{"notes/architecture.txt", "docs/specs.json", "image.png", "script.sh", ""}
	for _, path := range denied {
		if IsAllowedPath(path) {
			t.Errorf("expected denied: %s", path)
		}
	}
}

func TestIsAllowedPathHiddenFiles(t *testing.T) {
	denied := []string{".hidden.md", "notes/.draft.md", "docs/../secret.md"}
	for _, path := range denied {
		if IsAllowedPath(path) {
			t.Errorf("expected denied hidden: %s", path)
		}
	}
}

func TestIsAllowedPathSecretDirs(t *testing.T) {
	denied := []string{
		"node_modules/package.md",
		"vendor/dep.md",
		".git/config.md",
		"credentials/api.md",
		"secrets/keys.md",
		"keys/private.md",
		"certs/cert.md",
	}
	for _, path := range denied {
		if IsAllowedPath(path) {
			t.Errorf("expected denied secret dir: %s", path)
		}
	}
}

func TestRedactContentConvenience(t *testing.T) {
	input := "Email: test@example.com API_KEY=secret123"
	output := RedactContent(input)
	if strings.Contains(output, "test@example.com") || strings.Contains(output, "secret123") {
		t.Errorf("RedactContent failed: %q", output)
	}
	if !strings.Contains(output, "[EMAIL]") || !strings.Contains(output, "[REDACTED]") {
		t.Errorf("RedactContent missing markers: %q", output)
	}
}
