package printer

import "atol-server/internal/receipt"

type pixelBufferPrintOptions struct {
	pixels       []byte
	width        int
	alignment    receipt.Alignment
	scalePercent int
}

func pixelBufferFromReceiptLine(line receipt.Line) PixelBuffer {
	return PixelBuffer{
		Width:        line.ImageWidth,
		Height:       line.ImageHeight,
		Pixels:       append([]byte(nil), line.ImagePixelBuffer...),
		Alignment:    line.Alignment,
		ScalePercent: line.ImageScalePercent,
	}
}

func pixelBufferPrintParams(buffer PixelBuffer) pixelBufferPrintOptions {
	buffer = buffer.Normalized()
	return pixelBufferPrintOptions{
		pixels:       append([]byte(nil), buffer.Pixels...),
		width:        buffer.Width,
		alignment:    buffer.Alignment,
		scalePercent: buffer.ScalePercent,
	}
}
