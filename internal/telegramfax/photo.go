package telegramfax

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strings"
	"time"

	"atol-server/internal/printer"
	"atol-server/internal/receipt"
)

const (
	telegramPhotoFaxWidth     = 384
	telegramPhotoFaxMaxHeight = 2048
)

func bestPhotoSize(photos []PhotoSize) (PhotoSize, bool) {
	var best PhotoSize
	bestScore := int64(-1)
	for _, photo := range photos {
		if strings.TrimSpace(photo.FileID) == "" || photo.Width <= 0 || photo.Height <= 0 {
			continue
		}
		score := int64(photo.Width) * int64(photo.Height)
		if score > bestScore || (score == bestScore && photo.FileSize > best.FileSize) {
			best = photo
			bestScore = score
		}
	}
	return best, bestScore >= 0
}

func FormatPhotoReceiptLines(message Message, buffer printer.PixelBuffer, location *time.Location) []receipt.Line {
	if location == nil {
		location = time.Local
	}
	buffer = buffer.Normalized()
	printedAt := time.Unix(message.Date, 0).In(location)
	lines := []receipt.Line{
		{
			Text:         "INCOMING PHOTO FAX",
			Alignment:    receipt.AlignmentCenter,
			Role:         receipt.RoleNormal,
			DoubleWidth:  true,
			DoubleHeight: true,
		},
		{
			Text:      "From: " + senderDisplayName(message.From),
			Alignment: receipt.AlignmentCenter,
			Role:      receipt.RoleNormal,
		},
		{
			Text:      printedAt.Format("02.01.2006 15:04"),
			Alignment: receipt.AlignmentCenter,
			Role:      receipt.RoleNormal,
		},
		{
			Text:      "",
			Alignment: receipt.AlignmentCenter,
			Role:      receipt.RoleNormal,
		},
	}
	caption := strings.ReplaceAll(message.Caption, "\r\n", "\n")
	caption = strings.ReplaceAll(caption, "\r", "\n")
	if strings.TrimSpace(caption) != "" {
		for _, line := range strings.Split(caption, "\n") {
			lines = append(lines, receipt.Line{
				Text:      line,
				Alignment: receipt.AlignmentLeft,
				Role:      receipt.RoleNormal,
			})
		}
	}
	lines = append(lines, receipt.Line{
		ImageWidth:        buffer.Width,
		ImageHeight:       buffer.Height,
		ImagePixelBuffer:  append([]byte(nil), buffer.Pixels...),
		ImageScalePercent: buffer.ScalePercent,
		Alignment:         receipt.AlignmentCenter,
		Role:              receipt.RoleNormal,
	})
	lines = append(lines, receipt.Line{
		Text:      faxBottomSeparator,
		Alignment: receipt.AlignmentCenter,
		Role:      receipt.RoleNormal,
	})
	return lines
}

func PhotoBytesToPixelBuffer(data []byte) (printer.PixelBuffer, error) {
	if len(data) == 0 {
		return printer.PixelBuffer{}, fmt.Errorf("photo data is empty")
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return printer.PixelBuffer{}, fmt.Errorf("decode image: %w", err)
	}
	bounds := source.Bounds()
	sourceWidth := bounds.Dx()
	sourceHeight := bounds.Dy()
	if sourceWidth <= 0 || sourceHeight <= 0 {
		return printer.PixelBuffer{}, fmt.Errorf("image has invalid dimensions %dx%d", sourceWidth, sourceHeight)
	}
	targetWidth := telegramPhotoFaxWidth
	targetHeight := int(math.Round(float64(sourceHeight) * float64(targetWidth) / float64(sourceWidth)))
	if targetHeight < 1 {
		targetHeight = 1
	}
	if targetHeight > telegramPhotoFaxMaxHeight {
		targetHeight = telegramPhotoFaxMaxHeight
	}

	grayscale := make([]float64, targetWidth*targetHeight)
	for y := 0; y < targetHeight; y++ {
		sourceY := bounds.Min.Y + y*sourceHeight/targetHeight
		for x := 0; x < targetWidth; x++ {
			sourceX := bounds.Min.X + x*sourceWidth/targetWidth
			grayscale[y*targetWidth+x] = luminanceOnWhite(source.At(sourceX, sourceY))
		}
	}

	buffer := printer.PixelBuffer{
		Width:        targetWidth,
		Height:       targetHeight,
		Pixels:       ditherMonochrome(grayscale, targetWidth, targetHeight),
		Alignment:    receipt.AlignmentCenter,
		ScalePercent: 100,
	}
	if err := buffer.Validate(); err != nil {
		return printer.PixelBuffer{}, err
	}
	return buffer, nil
}

func luminanceOnWhite(value color.Color) float64 {
	red, green, blue, alpha := value.RGBA()
	const max = 0xffff
	if alpha < max {
		red = blendPremultipliedOnWhite(red, alpha)
		green = blendPremultipliedOnWhite(green, alpha)
		blue = blendPremultipliedOnWhite(blue, alpha)
	}
	return float64(red)/257*0.299 + float64(green)/257*0.587 + float64(blue)/257*0.114
}

func blendPremultipliedOnWhite(value uint32, alpha uint32) uint32 {
	const max = 0xffff
	blended := value + max - alpha
	if blended > max {
		return max
	}
	return blended
}

func ditherMonochrome(grayscale []float64, width int, height int) []byte {
	work := append([]float64(nil), grayscale...)
	pixels := make([]byte, len(work))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			index := y*width + x
			black := work[index] < 128
			quantized := 255.0
			if black {
				quantized = 0
				pixels[index] = 255
			}
			err := work[index] - quantized
			if x+1 < width {
				work[index+1] += err * 7 / 16
			}
			if y+1 >= height {
				continue
			}
			if x > 0 {
				work[index+width-1] += err * 3 / 16
			}
			work[index+width] += err * 5 / 16
			if x+1 < width {
				work[index+width+1] += err / 16
			}
		}
	}
	return pixels
}
