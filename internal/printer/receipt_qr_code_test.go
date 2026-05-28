package printer

import (
	"strings"
	"testing"

	"atol-server/internal/receipt"
)

func TestQRCodeFromReceiptLineRendersPixelBuffer(t *testing.T) {
	line := receipt.Line{
		QRCode:            "  http://192.168.0.25:8080/snapshots/abc  ",
		Alignment:         receipt.AlignmentRight,
		ImageScalePercent: 120,
	}

	buffer, err := qrCodePixelBufferFromReceiptLine(line)
	if err != nil {
		t.Fatalf("render QR pixel buffer: %v", err)
	}

	if buffer.Width <= 0 || buffer.Height <= 0 || buffer.Width != buffer.Height {
		t.Fatalf("expected square QR buffer, got %dx%d", buffer.Width, buffer.Height)
	}
	if buffer.Alignment != receipt.AlignmentRight {
		t.Fatalf("expected right alignment, got %q", buffer.Alignment)
	}
	if buffer.ScalePercent != 100 {
		t.Fatalf("expected QR bitmap to print at native scale, got %d", buffer.ScalePercent)
	}
	if err := buffer.Validate(); err != nil {
		t.Fatalf("expected valid pixel buffer: %v", err)
	}
	if countPixelValue(buffer.Pixels, 0) == 0 {
		t.Fatal("expected QR bitmap to contain black pixels")
	}
	if countPixelValue(buffer.Pixels, 255) == 0 {
		t.Fatal("expected QR bitmap to contain white pixels")
	}
}

func TestQRCodeFromReceiptLineSupportsSnapshotURLs(t *testing.T) {
	longURL := "http://192.168.0.25:8080/snapshots/" + strings.Repeat("abcdef1234567890", 6)

	buffer, err := qrCodePixelBufferFromReceiptLine(receipt.Line{QRCode: longURL})
	if err != nil {
		t.Fatalf("render long QR URL: %v", err)
	}

	if buffer.Width > maxQRCodePixelWidth {
		t.Fatalf("expected QR width to fit printer, got %d", buffer.Width)
	}
	if err := buffer.Validate(); err != nil {
		t.Fatalf("expected valid pixel buffer: %v", err)
	}
}

func TestQRCodeFromReceiptLineRejectsEmptyValue(t *testing.T) {
	if _, err := qrCodePixelBufferFromReceiptLine(receipt.Line{QRCode: "  "}); err == nil {
		t.Fatal("expected empty QR code error")
	}
}

func countPixelValue(pixels []byte, value byte) int {
	count := 0
	for _, pixel := range pixels {
		if pixel == value {
			count++
		}
	}
	return count
}
