package receipt

import "time"

type Alignment string
type Role string

const (
	AlignmentLeft   Alignment = "left"
	AlignmentCenter Alignment = "center"
	AlignmentRight  Alignment = "right"
)

const (
	RoleNormal      Role = "normal"
	RoleCalendar    Role = "calendar"
	RoleOriginal    Role = "original"
	RoleTemperature Role = "temperature"
)

type Line struct {
	Text              string
	ImageKey          string
	ImagePath         string
	ImageURL          string
	ImageWidth        int
	ImageHeight       int
	ImagePixelBuffer  []byte `json:"-"`
	ImageScalePercent int
	Alignment         Alignment
	Role              Role
	Font              int
	DoubleWidth       bool
	DoubleHeight      bool
}

type Image struct {
	Key          string
	Path         string
	URL          string
	Width        int
	Height       int
	PixelBuffer  []byte `json:"-"`
	ScalePercent int
}

func TestReceipt(printedAt time.Time) []Line {
	lines := []string{
		"Тестовая печать",
		printedAt.Format("02.01.2006 15:04"),
		"ATOL Go Server",
		"Wi-Fi TCP/IP",
		"Если чек вышел, печать работает.",
	}

	result := make([]Line, 0, len(lines))
	for _, line := range lines {
		result = append(result, Line{
			Text:      line,
			Alignment: AlignmentCenter,
			Role:      RoleNormal,
		})
	}
	return result
}
