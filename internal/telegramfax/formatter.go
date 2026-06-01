package telegramfax

import (
	"fmt"
	"strings"
	"time"

	"atol-server/internal/receipt"
)

func FormatReceiptLines(message Message, location *time.Location) []receipt.Line {
	if location == nil {
		location = time.Local
	}
	printedAt := time.Unix(message.Date, 0).In(location)
	text := strings.ReplaceAll(message.Text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := []receipt.Line{
		{
			Text:         "INCOMING FAX",
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
	for _, line := range strings.Split(text, "\n") {
		lines = append(lines, receipt.Line{
			Text:      line,
			Alignment: receipt.AlignmentLeft,
			Role:      receipt.RoleNormal,
		})
	}
	return lines
}

func senderDisplayName(user *User) string {
	if user == nil {
		return "Unknown sender"
	}
	var parts []string
	if strings.TrimSpace(user.FirstName) != "" {
		parts = append(parts, strings.TrimSpace(user.FirstName))
	}
	if strings.TrimSpace(user.LastName) != "" {
		parts = append(parts, strings.TrimSpace(user.LastName))
	}
	if len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("Telegram user %d", user.ID))
	}
	if strings.TrimSpace(user.Username) != "" {
		parts = append(parts, "@"+strings.TrimSpace(user.Username))
	}
	return strings.Join(parts, " ")
}
