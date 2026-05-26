package receipt

import (
	"testing"
	"time"
)

func TestTestReceiptUsesStableRussianText(t *testing.T) {
	printedAt := time.Date(2026, 5, 25, 9, 7, 0, 0, time.UTC)

	lines := TestReceipt(printedAt)

	texts := texts(lines)
	want := []string{
		"Тестовая печать",
		"25.05.2026 09:07",
		"ATOL Go Server",
		"Wi-Fi TCP/IP",
		"Если чек вышел, печать работает.",
	}

	if len(texts) != len(want) {
		t.Fatalf("expected %d lines, got %d: %#v", len(want), len(texts), texts)
	}
	for i := range want {
		if texts[i] != want[i] {
			t.Fatalf("line %d: expected %q, got %q", i, want[i], texts[i])
		}
	}
}

func TestTestReceiptCentersEveryLine(t *testing.T) {
	lines := TestReceipt(time.Date(2026, 5, 25, 9, 7, 0, 0, time.UTC))

	for _, line := range lines {
		if line.Alignment != AlignmentCenter {
			t.Fatalf("expected centered line, got %#v", line)
		}
	}
}

func texts(lines []Line) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		result = append(result, line.Text)
	}
	return result
}
