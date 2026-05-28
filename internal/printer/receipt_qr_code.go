package printer

import (
	"strings"

	"atol-server/internal/receipt"
)

type qrCodePrintOptions struct {
	value        string
	alignment    receipt.Alignment
	scalePercent int
}

func qrCodePrintParams(line receipt.Line) qrCodePrintOptions {
	scalePercent := line.ImageScalePercent
	if scalePercent <= 0 {
		scalePercent = 100
	}
	return qrCodePrintOptions{
		value:        strings.TrimSpace(line.QRCode),
		alignment:    line.Alignment,
		scalePercent: scalePercent,
	}
}
