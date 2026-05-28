package receiptsnapshot

import "testing"

func TestSettingsNormalizesBaseURL(t *testing.T) {
	settings := Settings{BaseURL: "  http://192.168.0.25:8080/  "}

	normalized := settings.Normalized()

	if normalized.BaseURL != "http://192.168.0.25:8080" {
		t.Fatalf("expected trimmed LAN base URL, got %q", normalized.BaseURL)
	}
}

func TestSettingsAllowsEmptyBaseURL(t *testing.T) {
	if err := (Settings{}).Validate(); err != nil {
		t.Fatalf("empty base URL should be allowed: %v", err)
	}
}

func TestSettingsRejectsInvalidBaseURL(t *testing.T) {
	settings := Settings{BaseURL: "localhost:8080"}

	if err := settings.Validate(); err == nil {
		t.Fatal("expected invalid base URL to be rejected")
	}
}

func TestSnapshotURLUsesNormalizedBaseURL(t *testing.T) {
	settings := Settings{BaseURL: "http://192.168.0.25:8080/"}

	got, ok := settings.SnapshotURL("abc-123")

	if !ok || got != "http://192.168.0.25:8080/snapshots/abc-123" {
		t.Fatalf("unexpected snapshot URL: got %q ok=%v", got, ok)
	}
}
