package printer

import (
	"path/filepath"
	"testing"
)

func TestConfigValidateAcceptsValidIPAddressAndPort(t *testing.T) {
	config := Config{Host: "192.168.0.118", Port: 5555}

	if err := config.Validate(); err != nil {
		t.Fatalf("expected config to be valid, got %v", err)
	}
}

func TestConfigValidateRejectsEmptyHost(t *testing.T) {
	config := Config{Host: " ", Port: 5555}

	if err := config.Validate(); err == nil {
		t.Fatal("expected empty host to be invalid")
	}
}

func TestConfigValidateRejectsInvalidIPAddress(t *testing.T) {
	config := Config{Host: "not an ip", Port: 5555}

	if err := config.Validate(); err == nil {
		t.Fatal("expected invalid IP address to be rejected")
	}
}

func TestConfigValidateRejectsInvalidPort(t *testing.T) {
	tests := []int{0, -1, 65536}

	for _, port := range tests {
		config := Config{Host: "192.168.0.118", Port: port}
		if err := config.Validate(); err == nil {
			t.Fatalf("expected port %d to be invalid", port)
		}
	}
}

func TestConfigNormalizedTrimsHost(t *testing.T) {
	config := Config{Host: " 192.168.0.118 ", Port: 5555}

	normalized := config.Normalized()

	if normalized.Host != "192.168.0.118" {
		t.Fatalf("expected trimmed host, got %q", normalized.Host)
	}
}

func TestGatewayWeatherIconPathUsesCuratedPrintableAssets(t *testing.T) {
	gateway := NewGateway("/opt/atol/lib", "/opt/app/assets")

	path, err := gateway.weatherIconPath("partly_cloudy")
	if err != nil {
		t.Fatalf("expected icon path, got %v", err)
	}

	want := filepath.Join("/opt/app/assets", "weather-icons", "print", "partly_cloudy.png")
	if path != want {
		t.Fatalf("expected %q, got %q", want, path)
	}
}

func TestGatewayWeatherIconPathRejectsTraversal(t *testing.T) {
	gateway := NewGateway("/opt/atol/lib", "/opt/app/assets")

	if _, err := gateway.weatherIconPath("../clear"); err == nil {
		t.Fatal("expected path traversal key to be rejected")
	}
}
