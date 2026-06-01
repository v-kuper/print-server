package telegramfax

import (
	"reflect"
	"testing"
	"time"

	"atol-server/internal/receipt"
)

func TestFormatReceiptLinesPreservesTelegramMessageText(t *testing.T) {
	message := Message{
		Date: time.Date(2026, 6, 1, 15, 10, 0, 0, time.UTC).Unix(),
		From: &User{
			ID:        2001,
			FirstName: "Vitali",
			LastName:  "Kupratsevich",
			Username:  "vitalik",
		},
		Text: "Первая строка\r\n\rВторая строка\n\nТретья строка",
	}

	lines := FormatReceiptLines(message, time.UTC)

	want := []receipt.Line{
		{Text: "INCOMING FAX", Alignment: receipt.AlignmentCenter, Role: receipt.RoleNormal, DoubleWidth: true, DoubleHeight: true},
		{Text: "From: Vitali Kupratsevich @vitalik", Alignment: receipt.AlignmentCenter, Role: receipt.RoleNormal},
		{Text: "01.06.2026 15:10", Alignment: receipt.AlignmentCenter, Role: receipt.RoleNormal},
		{Text: "", Alignment: receipt.AlignmentCenter, Role: receipt.RoleNormal},
		{Text: "Первая строка", Alignment: receipt.AlignmentLeft, Role: receipt.RoleNormal},
		{Text: "", Alignment: receipt.AlignmentLeft, Role: receipt.RoleNormal},
		{Text: "Вторая строка", Alignment: receipt.AlignmentLeft, Role: receipt.RoleNormal},
		{Text: "", Alignment: receipt.AlignmentLeft, Role: receipt.RoleNormal},
		{Text: "Третья строка", Alignment: receipt.AlignmentLeft, Role: receipt.RoleNormal},
	}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("expected %#v, got %#v", want, lines)
	}
}

func TestFormatReceiptLinesFallsBackToSenderID(t *testing.T) {
	message := Message{
		Date: time.Date(2026, 6, 1, 15, 10, 0, 0, time.UTC).Unix(),
		From: &User{ID: 2001},
		Text: "Hello",
	}

	lines := FormatReceiptLines(message, time.UTC)

	if lines[1].Text != "From: Telegram user 2001" {
		t.Fatalf("expected fallback sender, got %q", lines[1].Text)
	}
}
