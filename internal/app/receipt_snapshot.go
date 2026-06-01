package app

import (
	"context"
	"strings"

	"atol-server/internal/news"
	"atol-server/internal/receipt"
	"atol-server/internal/receiptsnapshot"
)

const defaultReceiptSnapshotBaseURL = "http://localhost:8080"

type receiptSnapshotFontMetric struct {
	lineLength int
	fontWidth  int
}

var receiptSnapshotFontMetrics = map[int]receiptSnapshotFontMetric{
	0: {lineLength: 32, fontWidth: 12},
	1: {lineLength: 42, fontWidth: 9},
	2: {lineLength: 38, fontWidth: 10},
	3: {lineLength: 32, fontWidth: 12},
	4: {lineLength: 32, fontWidth: 12},
	5: {lineLength: 32, fontWidth: 12},
	6: {lineLength: 32, fontWidth: 12},
	7: {lineLength: 32, fontWidth: 12},
	8: {lineLength: 32, fontWidth: 12},
	9: {lineLength: 32, fontWidth: 12},
}

func (s *ReceiptService) appendNewsSnapshotQRCode(ctx context.Context, lines []receipt.Line, items []news.Item, style receipt.StyleSettings, paperChars int) ([]receipt.Line, string, []string) {
	if (len(items) == 0 && !receiptLinesHaveLinks(lines)) || s.snapshotStore == nil {
		return lines, "", nil
	}
	settings, err := s.store.LoadReceiptSnapshotSettings()
	if err != nil {
		return lines, "", []string{"QR-ссылка на онлайн-слепок недоступна: " + err.Error()}
	}
	settings = settings.Normalized()
	if settings.BaseURL == "" {
		settings.BaseURL = defaultReceiptSnapshotBaseURL
	}
	if err := settings.Validate(); err != nil {
		return lines, "", []string{"QR-ссылка на онлайн-слепок некорректна: " + err.Error()}
	}
	snapshot, err := s.snapshotStore.Create(ctx, receiptsnapshot.CreateInput{
		NewsItems:    newsSnapshotItems(items),
		ReceiptLines: receiptSnapshotLines(lines, style),
		PaperChars:   paperChars,
	})
	if err != nil {
		return lines, "", []string{"Онлайн-слепок новостей не создан: " + err.Error()}
	}
	snapshotURL, ok := settings.SnapshotURL(snapshot.ID)
	if !ok {
		return lines, "", []string{"QR-ссылка на онлайн-слепок не настроена."}
	}
	finalLines := append([]receipt.Line(nil), lines...)
	finalLines = append(finalLines, qrCodeSectionSpacing(paperChars)...)
	finalLines = append(finalLines, receipt.Line{
		QRCode:            snapshotURL,
		Alignment:         receipt.AlignmentCenter,
		ImageScalePercent: 100,
	})
	if err := s.snapshotStore.FinalizeReceiptLines(ctx, snapshot.ID, receiptSnapshotLines(finalLines, style), paperChars); err != nil {
		return lines, "", []string{"Онлайн-слепок создан, но финальный чек не сохранен: " + err.Error()}
	}
	return finalLines, snapshot.ID, nil
}

func qrCodeSectionSpacing(paperChars int) []receipt.Line {
	separatorLength := paperChars / 2
	if separatorLength <= 0 {
		separatorLength = 16
	}
	return []receipt.Line{
		{Text: " ", Alignment: receipt.AlignmentCenter, Role: receipt.RoleNormal},
		{Text: strings.Repeat("-", separatorLength), Alignment: receipt.AlignmentCenter, Role: receipt.RoleNormal},
		{Text: " ", Alignment: receipt.AlignmentCenter, Role: receipt.RoleNormal},
	}
}

func receiptLinesHaveLinks(lines []receipt.Line) bool {
	for _, line := range lines {
		if strings.TrimSpace(line.Link) != "" {
			return true
		}
	}
	return false
}

func newsSnapshotItems(items []news.Item) []receiptsnapshot.NewsItem {
	result := make([]receiptsnapshot.NewsItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Title) == "" {
			continue
		}
		result = append(result, receiptsnapshot.NewsItem{
			Title:         strings.TrimSpace(item.Title),
			OriginalTitle: strings.TrimSpace(item.OriginalTitle),
			SourceName:    strings.TrimSpace(item.SourceName),
			Link:          strings.TrimSpace(item.Link),
		})
	}
	return result
}

func receiptSnapshotLines(lines []receipt.Line, style receipt.StyleSettings) []receiptsnapshot.ReceiptLine {
	style = style.Normalized()
	result := make([]receiptsnapshot.ReceiptLine, 0, len(lines))
	for _, line := range lines {
		result = append(result, receiptsnapshot.ReceiptLine{
			Text:              line.Text,
			Link:              line.Link,
			QRCode:            line.QRCode,
			ImageKey:          line.ImageKey,
			ImageURL:          line.ImageURL,
			ImageWidth:        line.ImageWidth,
			ImageHeight:       line.ImageHeight,
			ImageScalePercent: line.ImageScalePercent,
			Alignment:         string(line.Alignment),
			Role:              string(line.Role),
			Font:              line.Font,
			DoubleWidth:       line.DoubleWidth,
			DoubleHeight:      line.DoubleHeight,
			LineSize:          receiptSnapshotLineSize(line, style),
		})
	}
	return result
}

func receiptSnapshotPaperChars(style receipt.StyleSettings) int {
	metric := receiptSnapshotMetricForFont(style.Normalized().NormalFont)
	if metric.lineLength <= 0 {
		return 32
	}
	return metric.lineLength
}

func receiptSnapshotLineSize(line receipt.Line, style receipt.StyleSettings) float64 {
	normalMetric := receiptSnapshotMetricForFont(style.Normalized().NormalFont)
	metric := receiptSnapshotMetricForFont(line.Font)
	baseWidth := normalMetric.fontWidth
	if baseWidth <= 0 {
		baseWidth = 12
	}
	fontWidth := metric.fontWidth
	if fontWidth <= 0 {
		fontWidth = baseWidth
	}
	lineSize := 16 * (float64(fontWidth) / float64(baseWidth))
	if lineSize < 10 {
		return 10
	}
	if lineSize > 32 {
		return 32
	}
	return lineSize
}

func receiptSnapshotMetricForFont(font int) receiptSnapshotFontMetric {
	if metric, ok := receiptSnapshotFontMetrics[font]; ok {
		return metric
	}
	return receiptSnapshotFontMetrics[0]
}
