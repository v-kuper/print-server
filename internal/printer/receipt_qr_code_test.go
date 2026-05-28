package printer

import (
	"testing"

	"atol-server/internal/receipt"
)

func TestQRCodeFromReceiptLineNormalizesPrintParams(t *testing.T) {
	line := receipt.Line{
		QRCode:            "  http://192.168.0.25:8080/snapshots/abc  ",
		Alignment:         receipt.AlignmentRight,
		ImageScalePercent: 120,
	}

	params := qrCodePrintParams(line)

	if params.value != "http://192.168.0.25:8080/snapshots/abc" {
		t.Fatalf("expected trimmed QR value, got %q", params.value)
	}
	if params.alignment != receipt.AlignmentRight {
		t.Fatalf("expected right alignment, got %q", params.alignment)
	}
	if params.scalePercent != 120 {
		t.Fatalf("expected explicit scale percent, got %d", params.scalePercent)
	}
}

func TestQRCodeFromReceiptLineUsesDefaultScalePercent(t *testing.T) {
	params := qrCodePrintParams(receipt.Line{QRCode: "https://example.com/snapshots/abc"})

	if params.scalePercent != 100 {
		t.Fatalf("expected default scale percent, got %d", params.scalePercent)
	}
}
