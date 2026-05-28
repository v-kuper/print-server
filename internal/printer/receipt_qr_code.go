package printer

import (
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"

	"atol-server/internal/receipt"
)

const defaultQRCodeModuleScale = 6
const minQRCodeModuleScale = 2
const maxQRCodePixelWidth = 300

func qrCodePixelBufferFromReceiptLine(line receipt.Line) (PixelBuffer, error) {
	value := strings.TrimSpace(line.QRCode)
	if value == "" {
		return PixelBuffer{}, fmt.Errorf("QR code value is empty")
	}

	code, err := qrcode.New(value, qrcode.Medium)
	if err != nil {
		return PixelBuffer{}, fmt.Errorf("render QR code: %w", err)
	}
	bitmap := code.Bitmap()
	if len(bitmap) == 0 || len(bitmap[0]) == 0 {
		return PixelBuffer{}, fmt.Errorf("render QR code: empty bitmap")
	}

	scale := qrCodeModuleScale(len(bitmap))
	if scale < minQRCodeModuleScale {
		return PixelBuffer{}, fmt.Errorf("QR code is too dense for receipt printing")
	}

	width := len(bitmap[0]) * scale
	height := len(bitmap) * scale
	pixels := make([]byte, width*height)
	for row, modules := range bitmap {
		for col, dark := range modules {
			value := byte(255)
			if dark {
				value = 0
			}
			for y := 0; y < scale; y++ {
				offset := (row*scale+y)*width + col*scale
				for x := 0; x < scale; x++ {
					pixels[offset+x] = value
				}
			}
		}
	}

	return PixelBuffer{
		Width:        width,
		Height:       height,
		Pixels:       pixels,
		Alignment:    line.Alignment,
		ScalePercent: 100,
	}, nil
}

func qrCodeModuleScale(moduleCount int) int {
	if moduleCount <= 0 {
		return 0
	}
	scale := defaultQRCodeModuleScale
	if moduleCount*scale > maxQRCodePixelWidth {
		scale = maxQRCodePixelWidth / moduleCount
	}
	return scale
}
