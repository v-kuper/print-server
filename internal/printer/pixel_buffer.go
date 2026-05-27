package printer

import (
	"fmt"

	"atol-server/internal/receipt"
)

type PixelBuffer struct {
	Width        int
	Height       int
	Pixels       []byte
	Alignment    receipt.Alignment
	ScalePercent int
}

func (b PixelBuffer) Normalized() PixelBuffer {
	normalized := b.Clone()
	if normalized.Alignment == "" {
		normalized.Alignment = receipt.AlignmentCenter
	}
	if normalized.ScalePercent <= 0 {
		normalized.ScalePercent = 100
	}
	return normalized
}

func (b PixelBuffer) Validate() error {
	if b.Width <= 0 {
		return fmt.Errorf("pixel buffer width must be positive")
	}
	if b.Height <= 0 {
		return fmt.Errorf("pixel buffer height must be positive")
	}
	if len(b.Pixels) != b.Width*b.Height {
		return fmt.Errorf("pixel buffer length must be width * height, got %d for %dx%d", len(b.Pixels), b.Width, b.Height)
	}
	for index, value := range b.Pixels {
		if value != 0 && value != 255 {
			return fmt.Errorf("pixel value at index %d must be 0 or 255, got %d", index, value)
		}
	}
	return nil
}

func (b PixelBuffer) Clone() PixelBuffer {
	clone := b
	clone.Pixels = append([]byte(nil), b.Pixels...)
	return clone
}
