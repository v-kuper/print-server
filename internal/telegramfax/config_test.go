package telegramfax

import (
	"strings"
	"testing"
	"time"
)

func TestConfigFromEnvMissingTokenDisablesService(t *testing.T) {
	config, enabled, err := ConfigFromEnv(mapEnv(nil))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if enabled {
		t.Fatalf("expected service to be disabled without token")
	}
	if config.Token != "" {
		t.Fatalf("expected empty token, got %q", config.Token)
	}
}

func TestConfigFromEnvParsesIDsAndOptions(t *testing.T) {
	config, enabled, err := ConfigFromEnv(mapEnv(map[string]string{
		"TELEGRAM_FAX_BOT_TOKEN":            " 123:abc ",
		"TELEGRAM_FAX_OWNER_IDS":            "1001, 1002 1003",
		"TELEGRAM_FAX_ALLOWED_SENDER_IDS":   "2001 2002,2003",
		"TELEGRAM_FAX_API_BASE_URL":         " https://telegram.test/ ",
		"TELEGRAM_FAX_POLL_TIMEOUT_SECONDS": "9",
	}))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !enabled {
		t.Fatalf("expected service to be enabled")
	}
	if config.Token != "123:abc" {
		t.Fatalf("expected trimmed token, got %q", config.Token)
	}
	if config.APIBaseURL != "https://telegram.test" {
		t.Fatalf("expected trimmed API base URL, got %q", config.APIBaseURL)
	}
	if config.PollTimeout != 9*time.Second {
		t.Fatalf("expected 9s timeout, got %s", config.PollTimeout)
	}
	for _, id := range []int64{1001, 1002, 1003} {
		if !config.OwnerIDs.Contains(id) {
			t.Fatalf("expected owner ID %d to be allowed", id)
		}
	}
	for _, id := range []int64{2001, 2002, 2003} {
		if !config.AllowedSenderIDs.Contains(id) {
			t.Fatalf("expected sender ID %d to be allowed", id)
		}
	}
}

func TestConfigFromEnvRejectsInvalidIDs(t *testing.T) {
	_, enabled, err := ConfigFromEnv(mapEnv(map[string]string{
		"TELEGRAM_FAX_BOT_TOKEN":          "123:abc",
		"TELEGRAM_FAX_OWNER_IDS":          "1001,nope",
		"TELEGRAM_FAX_ALLOWED_SENDER_IDS": "2001",
	}))
	if err == nil {
		t.Fatalf("expected invalid ID error")
	}
	if !enabled {
		t.Fatalf("token is present, so enabled should still be reported")
	}
	if !strings.Contains(err.Error(), "TELEGRAM_FAX_OWNER_IDS") {
		t.Fatalf("expected env key in error, got %v", err)
	}
}

func TestConfigFromEnvRequiresOwnerAllowlistWhenEnabled(t *testing.T) {
	_, _, err := ConfigFromEnv(mapEnv(map[string]string{
		"TELEGRAM_FAX_BOT_TOKEN":          "123:abc",
		"TELEGRAM_FAX_ALLOWED_SENDER_IDS": "2001",
	}))
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "TELEGRAM_FAX_OWNER_IDS") {
		t.Fatalf("expected TELEGRAM_FAX_OWNER_IDS in error, got %v", err)
	}
}

func TestConfigFromEnvAllowsEmptySenderAllowlist(t *testing.T) {
	config, enabled, err := ConfigFromEnv(mapEnv(map[string]string{
		"TELEGRAM_FAX_BOT_TOKEN": "123:abc",
		"TELEGRAM_FAX_OWNER_IDS": "1001",
	}))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !enabled {
		t.Fatalf("expected service to be enabled")
	}
	if len(config.AllowedSenderIDs) != 0 {
		t.Fatalf("expected empty sender allowlist, got %#v", config.AllowedSenderIDs)
	}
}

func mapEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
