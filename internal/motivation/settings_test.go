package motivation

import "testing"

func TestSettingsNormalizeDefaults(t *testing.T) {
	settings := Settings{}.Normalized()

	if !settings.Enabled {
		t.Fatal("expected motivation quotes to be enabled by default")
	}
	if settings.BaseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, settings.BaseURL)
	}
	if settings.Model != DefaultModel {
		t.Fatalf("expected default model %q, got %q", DefaultModel, settings.Model)
	}
}

func TestSettingsNormalizeKeepsLegacyEnabledFlagTrue(t *testing.T) {
	settings := Settings{
		Configured: true,
		Enabled:    false,
		BaseURL:    DefaultBaseURL,
		Model:      DefaultModel,
	}.Normalized()

	if !settings.Enabled {
		t.Fatal("expected legacy enabled flag to normalize to true")
	}
}

func TestSettingsRejectInvalidBaseURL(t *testing.T) {
	settings := DefaultSettings()
	settings.BaseURL = "://bad"

	if err := settings.Validate(); err == nil {
		t.Fatal("expected invalid base URL to be rejected")
	}
}
