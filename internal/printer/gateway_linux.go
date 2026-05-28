//go:build linux

package printer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	fptr10 "atol-server/internal/fptr10"
	"atol-server/internal/receipt"
)

const receiptEndFeedLines = 4
const maxFontMetricsProbe = 9

func (g *Gateway) DriverVersion() (string, error) {
	fptr, err := fptr10.NewWithPath(g.LibraryPath)
	if err != nil {
		return "", fmt.Errorf("load ATOL driver: %w", err)
	}
	defer fptr.Destroy()
	return fptr.Version(), nil
}

func (g *Gateway) CheckConnection(ctx context.Context, config Config) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	session, err := g.open(config)
	if err != nil {
		return "", err
	}
	defer session.close()

	return "Подключено. Драйвер ATOL: " + session.fptr.Version(), nil
}

func (g *Gateway) PrintReceipt(ctx context.Context, config Config, lines []receipt.Line) error {
	if len(lines) == 0 {
		return fmt.Errorf("receipt has no lines to print")
	}
	session, err := g.open(config)
	if err != nil {
		return err
	}
	defer session.close()

	for _, line := range lines {
		if err := ctx.Err(); err != nil {
			return err
		}
		if strings.TrimSpace(line.QRCode) != "" {
			buffer, err := qrCodePixelBufferFromReceiptLine(line)
			if err != nil {
				return err
			}
			if err := session.printPixelBuffer(buffer); err != nil {
				return err
			}
		} else if len(line.ImagePixelBuffer) > 0 {
			buffer := pixelBufferFromReceiptLine(line).Normalized()
			if err := buffer.Validate(); err != nil {
				return err
			}
			if err := session.printPixelBuffer(buffer); err != nil {
				return err
			}
		} else if line.ImageKey != "" || line.ImagePath != "" {
			imagePath, err := g.imagePath(line.ImageKey, line.ImagePath)
			if err != nil {
				return err
			}
			if err := session.printPicture(imagePath, line); err != nil {
				return err
			}
		} else {
			if err := session.printLine(line); err != nil {
				return err
			}
		}
	}
	for range receiptEndFeedLines {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := session.lineFeed(); err != nil {
			return err
		}
	}
	return nil
}

func (g *Gateway) PrintPixelBuffer(ctx context.Context, config Config, buffer PixelBuffer) error {
	buffer = buffer.Normalized()
	if err := buffer.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	session, err := g.open(config)
	if err != nil {
		return err
	}
	defer session.close()

	if err := session.printPixelBuffer(buffer); err != nil {
		return err
	}
	for range receiptEndFeedLines {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := session.lineFeed(); err != nil {
			return err
		}
	}
	return nil
}

func (g *Gateway) FontMetrics(ctx context.Context, config Config) ([]FontMetric, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	session, err := g.open(config)
	if err != nil {
		return nil, err
	}
	defer session.close()

	var metrics []FontMetric
	var lastErr error
	for font := 0; font <= maxFontMetricsProbe; font++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		metric, err := session.fontMetric(font)
		if err != nil {
			lastErr = err
			session.fptr.ResetError()
			continue
		}
		metrics = append(metrics, metric)
	}
	if len(metrics) == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("font metrics are unavailable")
	}
	return metrics, nil
}

type fptrSession struct {
	fptr *fptr10.IFptr
}

func (g *Gateway) open(config Config) (*fptrSession, error) {
	normalized := config.Normalized()
	if err := normalized.Validate(); err != nil {
		return nil, err
	}

	fptr, err := fptr10.NewWithPath(g.LibraryPath)
	if err != nil {
		return nil, fmt.Errorf("load ATOL driver: %w", err)
	}

	session := &fptrSession{fptr: fptr}
	fptr.SetSingleSetting(fptr10.LIBFPTR_SETTING_MODEL, strconv.Itoa(fptr10.LIBFPTR_MODEL_ATOL_AUTO))
	fptr.SetSingleSetting(fptr10.LIBFPTR_SETTING_PORT, strconv.Itoa(fptr10.LIBFPTR_PORT_TCPIP))
	fptr.SetSingleSetting(fptr10.LIBFPTR_SETTING_IPADDRESS, normalized.Host)
	fptr.SetSingleSetting(fptr10.LIBFPTR_SETTING_IPPORT, strconv.Itoa(normalized.Port))
	fptr.SetSingleSetting(fptr10.LIBFPTR_SETTING_AUTO_RECONNECT, "1")
	fptr.SetSingleSetting(fptr10.LIBFPTR_SETTING_REMOTE_TIMEOUT, "30000")

	if err := session.call("apply settings", fptr.ApplySingleSettings); err != nil {
		fptr.Destroy()
		return nil, err
	}
	if err := session.call("open connection", fptr.Open); err != nil {
		fptr.Destroy()
		return nil, err
	}

	return session, nil
}

func (s *fptrSession) close() {
	if s.fptr == nil {
		return
	}
	if s.fptr.IsOpened() {
		_ = s.fptr.Close()
	}
	s.fptr.Destroy()
}

func (s *fptrSession) printLine(line receipt.Line) error {
	if err := s.call("reset print params", s.fptr.ResetParams); err != nil {
		return err
	}

	s.fptr.SetParam(fptr10.LIBFPTR_PARAM_TEXT, line.Text)
	s.fptr.SetParam(fptr10.LIBFPTR_PARAM_FONT, line.Font)
	s.fptr.SetParam(fptr10.LIBFPTR_PARAM_ALIGNMENT, alignmentToFptr(line.Alignment))
	s.fptr.SetParam(fptr10.LIBFPTR_PARAM_TEXT_WRAP, fptr10.LIBFPTR_TW_WORDS)
	s.fptr.SetParam(fptr10.LIBFPTR_PARAM_FONT_DOUBLE_WIDTH, line.DoubleWidth)
	s.fptr.SetParam(fptr10.LIBFPTR_PARAM_FONT_DOUBLE_HEIGHT, line.DoubleHeight)

	return s.call("print text", s.fptr.PrintText)
}

func (s *fptrSession) printPicture(path string, line receipt.Line) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("weather icon file %q is unavailable: %w", path, err)
	}
	if err := s.call("reset picture params", s.fptr.ResetParams); err != nil {
		return err
	}

	scalePercent := line.ImageScalePercent
	if scalePercent <= 0 {
		scalePercent = 100
	}
	s.fptr.SetParam(fptr10.LIBFPTR_PARAM_FILENAME, path)
	s.fptr.SetParam(fptr10.LIBFPTR_PARAM_ALIGNMENT, alignmentToFptr(line.Alignment))
	s.fptr.SetParam(fptr10.LIBFPTR_PARAM_SCALE_PERCENT, float64(scalePercent))

	return s.call("print picture", s.fptr.PrintPicture)
}

func (s *fptrSession) printPixelBuffer(buffer PixelBuffer) error {
	if err := s.call("reset pixel buffer params", s.fptr.ResetParams); err != nil {
		return err
	}

	params := pixelBufferPrintParams(buffer)
	s.fptr.SetParam(fptr10.LIBFPTR_PARAM_PIXEL_BUFFER, params.pixels)
	s.fptr.SetParam(fptr10.LIBFPTR_PARAM_WIDTH, params.width)
	s.fptr.SetParam(fptr10.LIBFPTR_PARAM_ALIGNMENT, alignmentToFptr(params.alignment))
	s.fptr.SetParam(fptr10.LIBFPTR_PARAM_SCALE_PERCENT, params.scalePercent)

	return s.call("print pixel buffer", s.fptr.PrintPixelBuffer)
}

func (s *fptrSession) lineFeed() error {
	return s.call("line feed", s.fptr.LineFeed)
}

func (s *fptrSession) fontMetric(font int) (FontMetric, error) {
	if err := s.call("reset font info params", s.fptr.ResetParams); err != nil {
		return FontMetric{}, err
	}

	s.fptr.SetParam(fptr10.LIBFPTR_PARAM_DATA_TYPE, fptr10.LIBFPTR_DT_FONT_INFO)
	s.fptr.SetParam(fptr10.LIBFPTR_PARAM_FONT, font)
	if err := s.call(fmt.Sprintf("query font %d info", font), s.fptr.QueryData); err != nil {
		return FontMetric{}, err
	}

	return FontMetric{
		Font:       font,
		LineLength: int(s.fptr.GetParamInt(fptr10.LIBFPTR_PARAM_RECEIPT_LINE_LENGTH)),
		FontWidth:  int(s.fptr.GetParamInt(fptr10.LIBFPTR_PARAM_FONT_WIDTH)),
	}, nil
}

func (s *fptrSession) call(operation string, call func() error) error {
	if err := call(); err != nil {
		return describeFptrError(operation, err)
	}
	return nil
}

func alignmentToFptr(alignment receipt.Alignment) int {
	switch alignment {
	case receipt.AlignmentLeft:
		return fptr10.LIBFPTR_ALIGNMENT_LEFT
	case receipt.AlignmentRight:
		return fptr10.LIBFPTR_ALIGNMENT_RIGHT
	default:
		return fptr10.LIBFPTR_ALIGNMENT_CENTER
	}
}

func describeFptrError(operation string, err error) error {
	var fptrErr *fptr10.Error
	if errors.As(err, &fptrErr) {
		return fmt.Errorf("%s failed: %s (%d)", operation, fptrErr.ErrorDescription, fptrErr.ErrorCode)
	}
	return fmt.Errorf("%s failed: %w", operation, err)
}
