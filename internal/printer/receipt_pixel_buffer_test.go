package printer

import (
	"testing"

	"atol-server/internal/receipt"
)

func TestPixelBufferFromReceiptLineUsesImagePixelBuffer(t *testing.T) {
	line := receipt.Line{
		ImageWidth:        384,
		ImageHeight:       2,
		ImagePixelBuffer:  append([]byte{255}, make([]byte, 767)...),
		ImageScalePercent: 120,
		Alignment:         receipt.AlignmentRight,
	}

	buffer := pixelBufferFromReceiptLine(line)

	if buffer.Width != 384 || buffer.Height != 2 {
		t.Fatalf("expected 384x2 buffer, got %#v", buffer)
	}
	if len(buffer.Pixels) != 768 || buffer.Pixels[0] != 255 {
		t.Fatalf("expected copied image pixel buffer, got %#v", buffer.Pixels[:min(len(buffer.Pixels), 4)])
	}
	if buffer.Alignment != receipt.AlignmentRight {
		t.Fatalf("expected line alignment to be preserved, got %q", buffer.Alignment)
	}
	if buffer.ScalePercent != 120 {
		t.Fatalf("expected line scale to be preserved, got %d", buffer.ScalePercent)
	}

	line.ImagePixelBuffer[0] = 0
	if buffer.Pixels[0] != 255 {
		t.Fatalf("expected pixel buffer copy to be isolated from receipt line")
	}
}

func TestPixelBufferPrintParamsUseIntegerScale(t *testing.T) {
	params := pixelBufferPrintParams(PixelBuffer{
		Width:        384,
		Height:       1,
		Pixels:       make([]byte, 384),
		ScalePercent: 100,
	})

	if params.scalePercent != 100 {
		t.Fatalf("expected integer scale percent, got %d", params.scalePercent)
	}
}
