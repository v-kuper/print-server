package printer

import "testing"

func TestPixelBufferValidateAcceptsMonochromePixels(t *testing.T) {
	buffer := PixelBuffer{
		Width:  4,
		Height: 2,
		Pixels: []byte{0, 255, 0, 255, 255, 0, 255, 0},
	}

	if err := buffer.Validate(); err != nil {
		t.Fatalf("expected pixel buffer to be valid, got %v", err)
	}
}

func TestPixelBufferValidateRejectsInvalidDimensionsAndPixels(t *testing.T) {
	tests := []struct {
		name   string
		buffer PixelBuffer
	}{
		{
			name:   "empty width",
			buffer: PixelBuffer{Width: 0, Height: 1, Pixels: []byte{0}},
		},
		{
			name:   "empty height",
			buffer: PixelBuffer{Width: 1, Height: 0, Pixels: []byte{0}},
		},
		{
			name:   "wrong length",
			buffer: PixelBuffer{Width: 4, Height: 2, Pixels: []byte{0, 255}},
		},
		{
			name:   "invalid pixel",
			buffer: PixelBuffer{Width: 2, Height: 1, Pixels: []byte{0, 42}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.buffer.Validate(); err == nil {
				t.Fatalf("expected invalid pixel buffer %#v to be rejected", tt.buffer)
			}
		})
	}
}

func TestPixelBufferCloneCopiesPixelBytes(t *testing.T) {
	buffer := PixelBuffer{Width: 2, Height: 1, Pixels: []byte{0, 255}}

	clone := buffer.Clone()
	clone.Pixels[0] = 255

	if buffer.Pixels[0] != 0 {
		t.Fatalf("expected clone to copy pixel bytes, got original %#v clone %#v", buffer.Pixels, clone.Pixels)
	}
}
