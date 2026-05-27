package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"atol-server/internal/finance"
	"atol-server/internal/googleintegration"
	"atol-server/internal/motivation"
	"atol-server/internal/news"
	"atol-server/internal/printer"
	"atol-server/internal/receipt"
	"atol-server/internal/schedule"
	schedulerruntime "atol-server/internal/scheduler"
	"atol-server/internal/weather"
)

func TestSavePrinterSettingsEndpointPersistsConfig(t *testing.T) {
	store := &fakeStore{config: printer.DefaultConfig()}
	server := NewServer(store, &fakePrinter{}, fixedClock)

	body := bytes.NewBufferString(`{"host":" 192.168.0.118 ","port":5555}`)
	request := httptest.NewRequest(http.MethodPost, "/api/settings/printer", body)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if store.config != (printer.Config{Host: "192.168.0.118", Port: 5555}) {
		t.Fatalf("expected saved normalized config, got %#v", store.config)
	}
}

func TestPrinterCheckEndpointUsesSavedConfig(t *testing.T) {
	store := &fakeStore{config: printer.Config{Host: "192.168.0.118", Port: 5555}}
	gateway := &fakePrinter{checkMessage: "Подключено. Драйвер ATOL: 10.10.8.0"}
	server := NewServer(store, gateway, fixedClock)

	request := httptest.NewRequest(http.MethodPost, "/api/printer/check", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if gateway.checkedConfig != store.config {
		t.Fatalf("expected check config %#v, got %#v", store.config, gateway.checkedConfig)
	}

	var payload statusResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Message != gateway.checkMessage {
		t.Fatalf("expected message %q, got %q", gateway.checkMessage, payload.Message)
	}
}

func TestPrinterFontsEndpointReturnsAtolFontMetrics(t *testing.T) {
	store := &fakeStore{config: printer.Config{Host: "192.168.0.118", Port: 5555}}
	gateway := &fakePrinter{fontMetrics: []printer.FontMetric{
		{Font: 0, LineLength: 32, FontWidth: 12},
		{Font: 1, LineLength: 42, FontWidth: 9},
		{Font: 2, LineLength: 24, FontWidth: 16},
	}}
	server := NewServer(store, gateway, fixedClock)

	request := httptest.NewRequest(http.MethodPost, "/api/printer/fonts", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if gateway.fontMetricsConfig != store.config {
		t.Fatalf("expected font metrics config %#v, got %#v", store.config, gateway.fontMetricsConfig)
	}

	var payload fontMetricsResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.OK {
		t.Fatalf("expected ok font metrics response, got %#v", payload)
	}
	if len(payload.Fonts) != 3 || payload.Fonts[1].LineLength != 42 {
		t.Fatalf("expected font metrics in response, got %#v", payload.Fonts)
	}
}

func TestIndexPageServesStaticClientShell(t *testing.T) {
	store := &fakeStore{config: printer.DefaultConfig()}
	server := NewServer(store, &fakePrinter{}, fixedClock)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected index page to disable browser cache, got %q", got)
	}
	if got := response.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("expected HTML content type, got %q", got)
	}
	body := response.Body.String()
	for _, want := range []string{
		`<link rel="stylesheet" href="/static/app.css">`,
		`<script src="/static/app.js" defer></script>`,
		`id="settings-form"`,
		`data-section="printer"`,
		`id="weather-location-results"`,
		`id="news-source-list"`,
		`id="schedule-time-list"`,
		`id="image-editor-file"`,
		`id="image-editor-canvas-height"`,
		`id="image-editor-result"`,
		`data-action="new-image-editor-canvas"`,
		`data-action="save-image-editor"`,
		`data-action="print-image-editor"`,
		`value="rectangle"`,
		`value="ellipse"`,
		`LIBFPTR_PARAM_FONT`,
		`data-action="weather"`,
	} {
		if !bytes.Contains([]byte(body), []byte(want)) {
			t.Fatalf("expected static client shell to contain %q", want)
		}
	}
	for _, unwanted := range []string{
		`{{`,
		`value="192.168.0.118"`,
		`id="content-calendar" type="checkbox" checked`,
	} {
		if bytes.Contains([]byte(body), []byte(unwanted)) {
			t.Fatalf("expected static client shell not to contain template data %q", unwanted)
		}
	}
}

func TestStaticClientAssetsServedWithoutCache(t *testing.T) {
	server := NewServer(&fakeStore{config: printer.DefaultConfig()}, &fakePrinter{}, fixedClock)

	for _, asset := range []struct {
		path        string
		contentType string
		contains    []string
	}{
		{
			path:        "/static/app.css",
			contentType: "text/css; charset=utf-8",
			contains: []string{
				`.layout { grid-template-columns: minmax(0, 1fr); }`,
				`.section-grid { grid-template-columns: minmax(0, 1fr); }`,
				`.weather-search-row { grid-template-columns: minmax(0, 1fr); }`,
				`[hidden]`,
				`display: none !important`,
				`overflow-wrap: anywhere`,
				`.primary-print {`,
			},
		},
		{
			path:        "/static/app.js",
			contentType: "text/javascript; charset=utf-8",
			contains: []string{
				`fetch("/api/bootstrap")`,
				`function applyBootstrap`,
				`data-action="preview"`,
				`data-action="google-disconnect"`,
				`/api/image-editor/save`,
				`/api/image-editor/print`,
				`function applyImageEditorProcessing`,
				`function createBlankImageEditorCanvas`,
				`function applyImageEditorShape`,
				`assetVersion`,
			},
		},
	} {
		request := httptest.NewRequest(http.MethodGet, asset.path, nil)
		response := httptest.NewRecorder()

		server.Routes().ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("expected %s to return 200, got %d: %s", asset.path, response.Code, response.Body.String())
		}
		if got := response.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("expected %s to disable browser cache, got %q", asset.path, got)
		}
		if got := response.Header().Get("Content-Type"); got != asset.contentType {
			t.Fatalf("expected %s content type %q, got %q", asset.path, asset.contentType, got)
		}
		body := response.Body.String()
		for _, want := range asset.contains {
			if !bytes.Contains([]byte(body), []byte(want)) {
				t.Fatalf("expected %s to contain %q", asset.path, want)
			}
		}
	}
}

func TestRuntimeAssetsServedWithoutCache(t *testing.T) {
	assetsPath := t.TempDir()
	iconDir := filepath.Join(assetsPath, "weather-icons", "print")
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		t.Fatalf("create icon dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(iconDir, "partly_cloudy.png"), []byte("png"), 0o644); err != nil {
		t.Fatalf("write icon: %v", err)
	}

	server := NewServer(&fakeStore{config: printer.DefaultConfig()}, &fakePrinter{}, fixedClock, WithAssetsPath(assetsPath))
	request := httptest.NewRequest(http.MethodGet, "/assets/weather-icons/print/partly_cloudy.png", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected asset 200, got %d: %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected runtime asset to disable browser cache, got %q", got)
	}
}

func TestBootstrapEndpointReturnsInitialClientState(t *testing.T) {
	translateTitles := false
	store := &fakeStore{
		config:   printer.Config{Host: "192.168.0.118", Port: 5555},
		location: weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
		portfolio: finance.TonPortfolio{
			AmountTon:   10.5,
			InvestedUSD: 20.25,
		},
		motivationSettings: motivation.Settings{
			Configured:  true,
			Enabled:     true,
			BaseURL:     "http://localhost:11434",
			Model:       "gemma4:31b-cloud",
			CachedQuote: "Последняя цитата",
		},
		newsSettings: news.Settings{
			TranslateTitles: &translateTitles,
			Sources: []news.SourceSettings{
				{Preset: news.PresetBBCRussian, Enabled: true, FeedURL: "https://example.com/rss", MaxItems: 3},
			},
		},
		receiptStyle: receipt.StyleSettings{
			Configured:              true,
			NormalFont:              1,
			CalendarFont:            2,
			TemperatureFont:         3,
			CalendarDoubleWidth:     true,
			CalendarDoubleHeight:    false,
			TemperatureDoubleWidth:  false,
			TemperatureDoubleHeight: true,
		},
		receiptContent: receipt.ContentSettings{
			Configured:          true,
			ShowWeather:         true,
			ShowWeatherAdvice:   false,
			ShowMotivationQuote: true,
			ShowTonPortfolio:    false,
			ShowUsdBynRate:      true,
			ShowBankRates:       false,
			ShowMail:            true,
			ShowCalendar:        true,
			ShowNews:            true,
		},
		scheduleSettings: schedule.Settings{
			Enabled:         true,
			Mode:            schedule.ModeDailyTimes,
			IntervalMinutes: 30,
			Times:           []string{"07:00", "09:00"},
			Timezone:        schedule.DefaultTimezone,
		},
	}
	googleClient := &fakeGoogleClient{status: googleintegration.Status{
		CredentialsAvailable: true,
		TokenAvailable:       false,
		Authorized:           false,
		CredentialsPath:      "/data/google/credentials.json",
		TokenPath:            "/data/google/token.json",
	}}
	server := NewServer(store, &fakePrinter{}, fixedClock, WithGoogleClient(googleClient))

	request := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("expected JSON content type, got %q", got)
	}

	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Printer      printer.Config           `json:"printer"`
			Weather      weather.Location         `json:"weather"`
			Finance      finance.TonPortfolio     `json:"finance"`
			Motivation   motivation.Settings      `json:"motivation"`
			GoogleStatus googleintegration.Status `json:"googleStatus"`
			News         struct {
				TranslateTitles bool                  `json:"translateTitles"`
				Sources         []news.SourceSettings `json:"sources"`
			} `json:"news"`
			ReceiptStyle      receipt.StyleSettings    `json:"receiptStyle"`
			ReceiptContent    receipt.ContentSettings  `json:"receiptContent"`
			Schedule          schedule.Settings        `json:"schedule"`
			NewsPresets       []news.PresetInfo        `json:"newsPresets"`
			FontOptions       []int                    `json:"fontOptions"`
			ScheduleIntervals []scheduleIntervalOption `json:"scheduleIntervals"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.OK {
		t.Fatalf("expected ok bootstrap response, got %#v", payload)
	}
	if payload.Data.Printer != store.config {
		t.Fatalf("expected printer config %#v, got %#v", store.config, payload.Data.Printer)
	}
	if payload.Data.Weather != store.location {
		t.Fatalf("expected weather location %#v, got %#v", store.location, payload.Data.Weather)
	}
	if payload.Data.Finance != store.portfolio {
		t.Fatalf("expected finance settings %#v, got %#v", store.portfolio, payload.Data.Finance)
	}
	if payload.Data.Motivation.BaseURL != "http://localhost:11434" || payload.Data.Motivation.Model != "gemma4:31b-cloud" {
		t.Fatalf("expected motivation settings in bootstrap, got %#v", payload.Data.Motivation)
	}
	if payload.Data.GoogleStatus.CredentialsPath != "/data/google/credentials.json" || payload.Data.GoogleStatus.Authorized {
		t.Fatalf("expected Google status in bootstrap, got %#v", payload.Data.GoogleStatus)
	}
	if payload.Data.News.TranslateTitles {
		t.Fatalf("expected disabled news translation in bootstrap")
	}
	if len(payload.Data.News.Sources) != len(news.Presets()) || payload.Data.News.Sources[0].MaxItems != 3 {
		t.Fatalf("expected normalized news sources in bootstrap, got %#v", payload.Data.News.Sources)
	}
	if len(payload.Data.NewsPresets) != len(news.Presets()) || payload.Data.NewsPresets[0].DisplayName == "" {
		t.Fatalf("expected news presets in bootstrap, got %#v", payload.Data.NewsPresets)
	}
	if payload.Data.ReceiptStyle.CalendarFont != 2 || !payload.Data.ReceiptStyle.CalendarDoubleWidth {
		t.Fatalf("expected receipt style in bootstrap, got %#v", payload.Data.ReceiptStyle)
	}
	if !payload.Data.ReceiptContent.ShowMail || payload.Data.ReceiptContent.ShowBankRates {
		t.Fatalf("expected receipt content in bootstrap, got %#v", payload.Data.ReceiptContent)
	}
	if payload.Data.Schedule.Mode != schedule.ModeDailyTimes || len(payload.Data.Schedule.Times) != 2 {
		t.Fatalf("expected schedule in bootstrap, got %#v", payload.Data.Schedule)
	}
	if len(payload.Data.FontOptions) != 10 || payload.Data.FontOptions[9] != 9 {
		t.Fatalf("expected font options 0..9, got %#v", payload.Data.FontOptions)
	}
	labels := make([]string, 0, len(payload.Data.ScheduleIntervals))
	for _, option := range payload.Data.ScheduleIntervals {
		labels = append(labels, option.Label)
	}
	for _, want := range []string{"1 минута", "5 минут", "30 минут", "1 час", "2 часа", "6 часов", "12 часов", "24 часа"} {
		if !containsString(labels, want) {
			t.Fatalf("expected schedule interval label %q in %#v", want, labels)
		}
	}
}

func TestImageEditorStateEndpointReturnsEmptyState(t *testing.T) {
	server := NewServer(
		&fakeStore{config: printer.DefaultConfig()},
		&fakePrinter{},
		fixedClock,
		WithImageEditorPath(t.TempDir()),
	)

	request := httptest.NewRequest(http.MethodGet, "/api/image-editor/state", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Available bool `json:"available"`
			Width     int  `json:"width"`
			Height    int  `json:"height"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.OK || payload.Data.Available || payload.Data.Width != 0 || payload.Data.Height != 0 {
		t.Fatalf("expected empty image editor state, got %#v", payload)
	}
}

func TestImageEditorSaveEndpointPersistsPixelBufferAndPreview(t *testing.T) {
	imageEditorPath := t.TempDir()
	server := NewServer(
		&fakeStore{config: printer.DefaultConfig()},
		&fakePrinter{},
		fixedClock,
		WithImageEditorPath(imageEditorPath),
	)

	request := httptest.NewRequest(http.MethodPost, "/api/image-editor/save", bytes.NewReader(imageEditorSaveJSON(t, 384, 1, nil)))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	buffer, err := os.ReadFile(filepath.Join(imageEditorPath, "current.bin"))
	if err != nil {
		t.Fatalf("read saved pixel buffer: %v", err)
	}
	if len(buffer) != 384 || buffer[0] != 0 || buffer[1] != 255 {
		t.Fatalf("expected persisted 384-byte 0/255 pixel buffer, got len=%d prefix=%#v", len(buffer), buffer[:min(len(buffer), 4)])
	}
	if _, err := os.Stat(filepath.Join(imageEditorPath, "current.json")); err != nil {
		t.Fatalf("expected metadata to be saved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(imageEditorPath, "preview.png")); err != nil {
		t.Fatalf("expected preview PNG to be saved: %v", err)
	}

	stateRequest := httptest.NewRequest(http.MethodGet, "/api/image-editor/state", nil)
	stateResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(stateResponse, stateRequest)
	if stateResponse.Code != http.StatusOK {
		t.Fatalf("expected state 200, got %d: %s", stateResponse.Code, stateResponse.Body.String())
	}
	var statePayload struct {
		OK   bool `json:"ok"`
		Data struct {
			Available  bool           `json:"available"`
			Width      int            `json:"width"`
			Height     int            `json:"height"`
			PreviewURL string         `json:"previewUrl"`
			Settings   map[string]any `json:"settings"`
		} `json:"data"`
	}
	if err := json.NewDecoder(stateResponse.Body).Decode(&statePayload); err != nil {
		t.Fatalf("decode state response: %v", err)
	}
	if !statePayload.OK || !statePayload.Data.Available || statePayload.Data.Width != 384 || statePayload.Data.Height != 1 {
		t.Fatalf("expected saved image editor state, got %#v", statePayload)
	}
	if statePayload.Data.PreviewURL != "/api/image-editor/preview" {
		t.Fatalf("expected preview URL, got %q", statePayload.Data.PreviewURL)
	}
	if statePayload.Data.Settings["dither"] != true {
		t.Fatalf("expected settings to round-trip, got %#v", statePayload.Data.Settings)
	}

	previewRequest := httptest.NewRequest(http.MethodGet, "/api/image-editor/preview", nil)
	previewResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(previewResponse, previewRequest)
	if previewResponse.Code != http.StatusOK {
		t.Fatalf("expected preview 200, got %d: %s", previewResponse.Code, previewResponse.Body.String())
	}
	if got := previewResponse.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected preview to disable cache, got %q", got)
	}
	if got := previewResponse.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("expected PNG content type, got %q", got)
	}
}

func TestImageEditorSaveEndpointRejectsInvalidPixelPayload(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		mutate    func([]int) []int
		wantError string
	}{
		{
			name:      "wrong width",
			width:     383,
			height:    1,
			wantError: "width must be 384",
		},
		{
			name:   "wrong length",
			width:  384,
			height: 1,
			mutate: func(pixels []int) []int {
				return pixels[:len(pixels)-1]
			},
			wantError: "pixel buffer length",
		},
		{
			name:   "invalid pixel value",
			width:  384,
			height: 1,
			mutate: func(pixels []int) []int {
				pixels[0] = 42
				return pixels
			},
			wantError: "pixel value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(
				&fakeStore{config: printer.DefaultConfig()},
				&fakePrinter{},
				fixedClock,
				WithImageEditorPath(t.TempDir()),
			)

			body := imageEditorSaveJSON(t, tt.width, tt.height, tt.mutate)
			request := httptest.NewRequest(http.MethodPost, "/api/image-editor/save", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()

			server.Routes().ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %s", tt.wantError, response.Body.String())
			}
		})
	}
}

func TestImageEditorPrintEndpointPrintsSavedPixelBuffer(t *testing.T) {
	imageEditorPath := t.TempDir()
	store := &fakeStore{config: printer.Config{Host: "192.168.0.118", Port: 5555}}
	gateway := &fakePrinter{}
	server := NewServer(store, gateway, fixedClock, WithImageEditorPath(imageEditorPath))

	saveRequest := httptest.NewRequest(http.MethodPost, "/api/image-editor/save", bytes.NewReader(imageEditorSaveJSON(t, 384, 1, nil)))
	saveRequest.Header.Set("Content-Type", "application/json")
	saveResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(saveResponse, saveRequest)
	if saveResponse.Code != http.StatusOK {
		t.Fatalf("expected save 200, got %d: %s", saveResponse.Code, saveResponse.Body.String())
	}

	printRequest := httptest.NewRequest(http.MethodPost, "/api/image-editor/print", nil)
	printResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(printResponse, printRequest)

	if printResponse.Code != http.StatusOK {
		t.Fatalf("expected print 200, got %d: %s", printResponse.Code, printResponse.Body.String())
	}
	if gateway.printedPixelBufferConfig != store.config {
		t.Fatalf("expected print config %#v, got %#v", store.config, gateway.printedPixelBufferConfig)
	}
	if gateway.printedPixelBuffer.Width != 384 || gateway.printedPixelBuffer.Height != 1 || len(gateway.printedPixelBuffer.Pixels) != 384 {
		t.Fatalf("expected printed 384x1 pixel buffer, got %#v", gateway.printedPixelBuffer)
	}
	if gateway.printedPixelBuffer.Pixels[1] != 255 {
		t.Fatalf("expected black pixel to round-trip, got %#v", gateway.printedPixelBuffer.Pixels[:4])
	}
}

func TestImageEditorClearEndpointDeletesSavedState(t *testing.T) {
	imageEditorPath := t.TempDir()
	server := NewServer(&fakeStore{config: printer.DefaultConfig()}, &fakePrinter{}, fixedClock, WithImageEditorPath(imageEditorPath))

	saveRequest := httptest.NewRequest(http.MethodPost, "/api/image-editor/save", bytes.NewReader(imageEditorSaveJSON(t, 384, 1, nil)))
	saveRequest.Header.Set("Content-Type", "application/json")
	saveResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(saveResponse, saveRequest)
	if saveResponse.Code != http.StatusOK {
		t.Fatalf("expected save 200, got %d: %s", saveResponse.Code, saveResponse.Body.String())
	}

	clearRequest := httptest.NewRequest(http.MethodDelete, "/api/image-editor/image", nil)
	clearResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(clearResponse, clearRequest)

	if clearResponse.Code != http.StatusOK {
		t.Fatalf("expected clear 200, got %d: %s", clearResponse.Code, clearResponse.Body.String())
	}
	for _, name := range []string{"current.bin", "current.json", "preview.png"} {
		if _, err := os.Stat(filepath.Join(imageEditorPath, name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, stat error: %v", name, err)
		}
	}
}

func TestSaveReceiptContentEndpointPersistsContent(t *testing.T) {
	store := &fakeStore{}
	server := NewServer(store, &fakePrinter{}, fixedClock)

	body := bytes.NewBufferString(`{"configured":true,"showWeather":false,"showWeatherAdvice":false,"showMotivationQuote":true,"showTonPortfolio":false,"showUsdBynRate":true,"showBankRates":false,"showMail":true,"showCalendar":false,"showNews":true}`)
	request := httptest.NewRequest(http.MethodPost, "/api/settings/receipt-content", body)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if store.receiptContent.ShowWeather ||
		store.receiptContent.ShowWeatherAdvice ||
		store.receiptContent.ShowTonPortfolio ||
		store.receiptContent.ShowBankRates ||
		store.receiptContent.ShowCalendar ||
		!store.receiptContent.ShowMotivationQuote ||
		!store.receiptContent.ShowUsdBynRate ||
		!store.receiptContent.ShowMail ||
		!store.receiptContent.ShowNews {
		t.Fatalf("expected saved receipt content toggles, got %#v", store.receiptContent)
	}
}

func TestIndexPagePlacesPreviewButtonInPreviewPanel(t *testing.T) {
	store := &fakeStore{config: printer.DefaultConfig()}
	server := NewServer(store, &fakePrinter{}, fixedClock)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	panelIndex := bytes.Index([]byte(body), []byte(`class="preview-panel"`))
	previewButtonIndex := bytes.Index([]byte(body), []byte(`data-action="preview"`))
	mainActionsIndex := bytes.Index([]byte(body), []byte(`<div class="actions main-actions">`))
	previewPanelEnd := -1
	if panelIndex >= 0 {
		previewPanelEnd = panelIndex + bytes.Index([]byte(body[panelIndex:]), []byte(`</aside>`))
	}
	if panelIndex < 0 || previewButtonIndex < 0 {
		t.Fatalf("expected preview panel and preview button in page")
	}
	if previewPanelEnd < panelIndex || previewButtonIndex < panelIndex || previewButtonIndex > previewPanelEnd {
		t.Fatalf("expected preview button to live inside preview panel")
	}
	if mainActionsIndex < 0 {
		t.Fatal("expected bottom action row to be marked")
	}
	mainActionsEnd := mainActionsIndex + bytes.Index([]byte(body[mainActionsIndex:]), []byte(`</div>`))
	if mainActionsEnd > mainActionsIndex && bytes.Contains([]byte(body[mainActionsIndex:mainActionsEnd]), []byte(`data-action="preview"`)) {
		t.Fatal("expected preview button to be removed from bottom action row")
	}
}

func TestIndexPagePlacesPrinterActionsInCashSection(t *testing.T) {
	store := &fakeStore{config: printer.DefaultConfig()}
	server := NewServer(store, &fakePrinter{}, fixedClock)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	cashSectionIndex := bytes.Index([]byte(body), []byte(`data-section="printer"`))
	checkButtonIndex := bytes.Index([]byte(body), []byte(`data-action="check"`))
	testPrintButtonIndex := bytes.Index([]byte(body), []byte(`data-action="print"`))
	mainActionsIndex := bytes.Index([]byte(body), []byte(`<div class="actions main-actions">`))
	if cashSectionIndex < 0 || checkButtonIndex < 0 || testPrintButtonIndex < 0 {
		t.Fatal("expected cash section, check button, and test receipt button in page")
	}
	cashSectionEnd := cashSectionIndex + bytes.Index([]byte(body[cashSectionIndex:]), []byte(`</section>`))
	if cashSectionEnd < cashSectionIndex || checkButtonIndex < cashSectionIndex || checkButtonIndex > cashSectionEnd {
		t.Fatal("expected check button to live inside cash section")
	}
	if testPrintButtonIndex < cashSectionIndex || testPrintButtonIndex > cashSectionEnd {
		t.Fatal("expected test receipt button to live inside cash section")
	}
	if mainActionsIndex < 0 {
		t.Fatal("expected bottom action row to be marked")
	}
	mainActionsEnd := mainActionsIndex + bytes.Index([]byte(body[mainActionsIndex:]), []byte(`</div>`))
	if mainActionsEnd > mainActionsIndex {
		mainActions := []byte(body[mainActionsIndex:mainActionsEnd])
		for _, action := range [][]byte{[]byte(`data-action="check"`), []byte(`data-action="print"`)} {
			if bytes.Contains(mainActions, action) {
				t.Fatalf("expected %s to be removed from bottom action row", action)
			}
		}
	}
}

func TestIndexPageMakesDailyPrintThePrimaryBottomAction(t *testing.T) {
	store := &fakeStore{config: printer.DefaultConfig()}
	server := NewServer(store, &fakePrinter{}, fixedClock)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	mainActionsIndex := bytes.Index([]byte(body), []byte(`<div class="actions main-actions">`))
	weatherButtonIndex := bytes.Index([]byte(body), []byte(`data-action="weather"`))
	if mainActionsIndex < 0 || weatherButtonIndex < 0 {
		t.Fatal("expected bottom action row and daily print button in page")
	}
	mainActionsEnd := mainActionsIndex + bytes.Index([]byte(body[mainActionsIndex:]), []byte(`</div>`))
	if mainActionsEnd < mainActionsIndex || weatherButtonIndex < mainActionsIndex || weatherButtonIndex > mainActionsEnd {
		t.Fatal("expected daily print button to live inside bottom action row")
	}
	mainActions := body[mainActionsIndex:mainActionsEnd]
	if !bytes.Contains([]byte(body), []byte(`class="primary-print" data-action="weather"`)) {
		t.Fatalf("expected index page to contain primary daily print button")
	}
	if bytes.Count([]byte(mainActions), []byte(`<button`)) != 1 {
		t.Fatalf("expected bottom action row to contain only daily print button, got %q", mainActions)
	}
}

func TestGoogleStatusEndpointReturnsClientStatus(t *testing.T) {
	googleClient := &fakeGoogleClient{status: googleintegration.Status{
		CredentialsAvailable: true,
		TokenAvailable:       true,
		Authorized:           true,
		CredentialsPath:      "/data/google/credentials.json",
		TokenPath:            "/data/google/token.json",
	}}
	server := NewServer(&fakeStore{}, &fakePrinter{}, fixedClock, WithGoogleClient(googleClient))

	request := httptest.NewRequest(http.MethodGet, "/api/google/status", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var payload googleStatusResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.OK || payload.Status == nil || !payload.Status.Authorized {
		t.Fatalf("expected authorized Google status, got %#v", payload)
	}
}

func TestGoogleAuthStartRedirectsWithLocalCallbackURL(t *testing.T) {
	googleClient := &fakeGoogleClient{authURL: "https://accounts.example/auth?ok=1"}
	server := NewServer(&fakeStore{}, &fakePrinter{}, fixedClock, WithGoogleClient(googleClient))

	request := httptest.NewRequest(http.MethodGet, "/api/google/auth/start", nil)
	request.Host = "localhost:8080"
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d: %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Location"); got != googleClient.authURL {
		t.Fatalf("expected redirect to %q, got %q", googleClient.authURL, got)
	}
	if googleClient.authRedirectURI != "http://localhost:8080/oauth/google/callback" {
		t.Fatalf("expected local callback URI, got %q", googleClient.authRedirectURI)
	}
}

func TestGoogleCallbackExchangesCode(t *testing.T) {
	googleClient := &fakeGoogleClient{}
	server := NewServer(&fakeStore{}, &fakePrinter{}, fixedClock, WithGoogleClient(googleClient))

	request := httptest.NewRequest(http.MethodGet, "/oauth/google/callback?code=abc123", nil)
	request.Host = "localhost:8080"
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if googleClient.exchangedCode != "abc123" {
		t.Fatalf("expected exchanged code abc123, got %q", googleClient.exchangedCode)
	}
	if googleClient.exchangeRedirectURI != "http://localhost:8080/oauth/google/callback" {
		t.Fatalf("expected exchange callback URI, got %q", googleClient.exchangeRedirectURI)
	}
}

func TestPrintTestEndpointPrintsStableTestReceipt(t *testing.T) {
	store := &fakeStore{config: printer.Config{Host: "192.168.0.118", Port: 5555}}
	gateway := &fakePrinter{}
	server := NewServer(store, gateway, fixedClock)

	request := httptest.NewRequest(http.MethodPost, "/api/print/test", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if gateway.printedConfig != store.config {
		t.Fatalf("expected print config %#v, got %#v", store.config, gateway.printedConfig)
	}
	want := receipt.TestReceipt(fixedClock())
	if len(gateway.printedLines) != len(want) {
		t.Fatalf("expected %d receipt lines, got %d", len(want), len(gateway.printedLines))
	}
	if gateway.printedLines[0].Text != "Тестовая печать" {
		t.Fatalf("expected test receipt title, got %q", gateway.printedLines[0].Text)
	}
}

func TestPrintWeatherEndpointLoadsWeatherAndPrintsReceipt(t *testing.T) {
	weatherCode := 0
	store := &fakeStore{
		config:    printer.Config{Host: "192.168.0.118", Port: 5555},
		location:  weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
		portfolio: finance.DefaultTonPortfolio(),
		newsSettings: news.Settings{Sources: []news.SourceSettings{
			{Preset: news.PresetBBCRussian, Enabled: true, FeedURL: "https://example.com/rss", MaxItems: 1},
		}},
	}
	gateway := &fakePrinter{}
	weatherProvider := &fakeWeatherProvider{
		snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		},
	}
	tonProvider := &fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}
	fiatProvider := &fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}
	newsProvider := &fakeNewsProvider{items: []news.Item{{Title: "Заголовок", SourceName: "BBC Russian"}}}
	server := NewServer(
		store,
		gateway,
		fixedClock,
		WithWeatherProvider(weatherProvider),
		WithTonPriceProvider(tonProvider),
		WithFiatRateProvider(fiatProvider),
		WithNewsProvider(newsProvider),
		WithMotivationProvider(&fakeMotivationProvider{
			quote:  motivation.Quote{Text: "Делай важное спокойно."},
			advice: motivation.WeatherAdvice{Text: "Возьми зонт."},
		}),
	)

	request := httptest.NewRequest(http.MethodPost, "/api/print/weather", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if weatherProvider.location != store.location {
		t.Fatalf("expected weather location %#v, got %#v", store.location, weatherProvider.location)
	}
	if gateway.printedConfig != store.config {
		t.Fatalf("expected print config %#v, got %#v", store.config, gateway.printedConfig)
	}
	if len(gateway.printedLines) == 0 {
		t.Fatal("expected weather receipt lines")
	}
	if gateway.printedLines[1].Text != "25 Мая" {
		t.Fatalf("expected calendar line, got %#v", gateway.printedLines)
	}
	if !lineTextsContain(gateway.printedLines, "TON") {
		t.Fatalf("expected TON block, got %#v", gateway.printedLines)
	}
	if !lineTextsContain(gateway.printedLines, "Курс доллара") {
		t.Fatalf("expected fiat block, got %#v", gateway.printedLines)
	}
	if !lineTextsContain(gateway.printedLines, "Коротко о мире:") {
		t.Fatalf("expected news block, got %#v", gateway.printedLines)
	}
	if !lineTextsContain(gateway.printedLines, "Делай важное спокойно.") {
		t.Fatalf("expected motivation quote, got %#v", gateway.printedLines)
	}
	if !lineTextsContain(gateway.printedLines, "Возьми зонт.") {
		t.Fatalf("expected weather advice, got %#v", gateway.printedLines)
	}
}

func TestReceiptPreviewEndpointReturnsStyledLinesWithoutPrinting(t *testing.T) {
	weatherCode := 0
	store := &fakeStore{
		config:    printer.Config{Host: "192.168.0.118", Port: 5555},
		location:  weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
		portfolio: finance.DefaultTonPortfolio(),
		newsSettings: news.Settings{Sources: []news.SourceSettings{
			{Preset: news.PresetBBCRussian, Enabled: true, FeedURL: "https://example.com/rss", MaxItems: 1},
		}},
		receiptStyle: receipt.StyleSettings{
			Configured:              true,
			NormalFont:              1,
			EmphasisFont:            2,
			CalendarDoubleWidth:     false,
			CalendarDoubleHeight:    true,
			TemperatureDoubleWidth:  true,
			TemperatureDoubleHeight: false,
		},
	}
	gateway := &fakePrinter{}
	server := NewServer(
		store,
		gateway,
		fixedClock,
		WithWeatherProvider(&fakeWeatherProvider{
			snapshot: weather.Snapshot{
				Timezone:     "Europe/Minsk",
				ObservedAt:   fixedClock(),
				TemperatureC: 22.2,
				WeatherCode:  &weatherCode,
			},
		}),
		WithTonPriceProvider(&fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithNewsProvider(&fakeNewsProvider{items: []news.Item{{Title: "Заголовок", SourceName: "BBC Russian"}}}),
		WithMotivationProvider(&fakeMotivationProvider{
			quote:  motivation.Quote{Text: "Делай важное спокойно."},
			advice: motivation.WeatherAdvice{Text: "Возьми зонт."},
		}),
	)

	request := httptest.NewRequest(http.MethodPost, "/api/receipt/preview", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if len(gateway.printedLines) != 0 {
		t.Fatalf("preview must not print, got %#v", gateway.printedLines)
	}

	var payload receiptPreviewResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.OK {
		t.Fatalf("expected ok preview response, got %#v", payload)
	}
	if len(payload.Lines) == 0 {
		t.Fatal("expected preview lines")
	}
	if payload.Lines[1].Text != "25 Мая" || payload.Lines[1].Role != receipt.RoleCalendar {
		t.Fatalf("expected styled calendar line, got %#v", payload.Lines[1])
	}
	if payload.Lines[1].Font != 2 || payload.Lines[1].DoubleWidth || !payload.Lines[1].DoubleHeight {
		t.Fatalf("expected calendar style from settings, got %#v", payload.Lines[1])
	}
	if !lineTextsContain(payload.Lines, "Коротко о мире:") {
		t.Fatalf("expected news in preview, got %#v", payload.Lines)
	}
	if !lineTextsContain(payload.Lines, "Делай важное спокойно.") {
		t.Fatalf("expected motivation quote in preview, got %#v", payload.Lines)
	}
	if !lineTextsContain(payload.Lines, "Возьми зонт.") {
		t.Fatalf("expected weather advice in preview, got %#v", payload.Lines)
	}
}

func TestSaveReceiptStyleEndpointPersistsStyle(t *testing.T) {
	store := &fakeStore{}
	server := NewServer(store, &fakePrinter{}, fixedClock)

	body := bytes.NewBufferString(`{"configured":true,"normalFont":1,"emphasisFont":2,"calendarDoubleWidth":false,"calendarDoubleHeight":true,"temperatureDoubleWidth":true,"temperatureDoubleHeight":false}`)
	request := httptest.NewRequest(http.MethodPost, "/api/settings/receipt-style", body)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if store.receiptStyle.NormalFont != 1 || store.receiptStyle.EmphasisFont != 2 {
		t.Fatalf("expected saved receipt style, got %#v", store.receiptStyle)
	}
}

func TestWeatherLocationSearchEndpointReturnsCandidates(t *testing.T) {
	provider := &fakeLocationSearchProvider{results: []weather.LocationCandidate{
		{
			Name:        "Гомель",
			Latitude:    52.4345,
			Longitude:   30.9754,
			Country:     "Беларусь",
			Admin1:      "Гомельская область",
			DisplayName: "Гомель, Гомельская область, Беларусь",
		},
	}}
	server := NewServer(&fakeStore{}, &fakePrinter{}, fixedClock, WithLocationSearchProvider(provider))

	request := httptest.NewRequest(http.MethodGet, "/api/weather/locations?q=%D0%93%D0%BE%D0%BC%D0%B5%D0%BB%D1%8C", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if provider.query != "Гомель" {
		t.Fatalf("expected query to be passed to provider, got %q", provider.query)
	}
	var payload locationSearchResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.OK || len(payload.Results) != 1 {
		t.Fatalf("expected location results, got %#v", payload)
	}
	if payload.Results[0].Latitude != 52.4345 || payload.Results[0].Longitude != 30.9754 {
		t.Fatalf("unexpected location result: %#v", payload.Results[0])
	}
}

func TestSaveNewsEndpointRejectsEnabledSourceWithoutCount(t *testing.T) {
	store := &fakeStore{}
	server := NewServer(store, &fakePrinter{}, fixedClock)

	body := bytes.NewBufferString(`{"sources":[{"preset":"bbc_russian","enabled":true,"feedUrl":"https://example.com/rss","maxItems":0}]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/settings/news", body)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
	}
}

func TestSaveNewsEndpointPersistsTranslateTitlesToggle(t *testing.T) {
	store := &fakeStore{}
	server := NewServer(store, &fakePrinter{}, fixedClock)

	body := bytes.NewBufferString(`{"translateTitles":false,"sources":[{"preset":"bbc_russian","enabled":true,"feedUrl":"https://example.com/rss","maxItems":1}]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/settings/news", body)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if store.newsSettings.TranslateTitles == nil || *store.newsSettings.TranslateTitles {
		t.Fatalf("expected disabled news translation setting to be saved, got %#v", store.newsSettings)
	}
}

func TestSaveMotivationEndpointPersistsSettings(t *testing.T) {
	store := &fakeStore{}
	server := NewServer(store, &fakePrinter{}, fixedClock)

	body := bytes.NewBufferString(`{"enabled":false,"baseUrl":" http://localhost:11434 ","model":" gemma4:31b-cloud "}`)
	request := httptest.NewRequest(http.MethodPost, "/api/settings/motivation", body)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if store.motivationSettings.BaseURL != "http://localhost:11434" || store.motivationSettings.Model != "gemma4:31b-cloud" {
		t.Fatalf("expected saved motivation settings, got %#v", store.motivationSettings)
	}
	if !store.motivationSettings.Enabled {
		t.Fatalf("expected legacy motivation enabled flag to be saved true, got %#v", store.motivationSettings)
	}
}

func TestMotivationTestEndpointRefreshesQuote(t *testing.T) {
	content := receipt.ContentSettings{
		Configured:          true,
		ShowWeather:         true,
		ShowWeatherAdvice:   true,
		ShowMotivationQuote: false,
		ShowTonPortfolio:    true,
		ShowUsdBynRate:      true,
		ShowBankRates:       true,
		ShowMail:            false,
		ShowCalendar:        true,
		ShowNews:            true,
	}
	store := &fakeStore{
		motivationSettings: motivation.Settings{Configured: true, Enabled: false},
		receiptContent:     content,
	}
	provider := &fakeMotivationProvider{quote: motivation.Quote{Text: "Сегодня достаточно одного шага."}}
	server := NewServer(store, &fakePrinter{}, fixedClock, WithMotivationProvider(provider))

	request := httptest.NewRequest(http.MethodPost, "/api/motivation/test", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var payload motivationResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.OK || payload.Quote == nil || payload.Quote.Text != "Сегодня достаточно одного шага." {
		t.Fatalf("expected quote payload, got %#v", payload)
	}
	if store.motivationSettings.CachedQuote != "Сегодня достаточно одного шага." {
		t.Fatalf("expected quote cache to be saved, got %#v", store.motivationSettings)
	}
	if !store.motivationSettings.Enabled {
		t.Fatalf("expected AI test to keep legacy enabled flag true, got %#v", store.motivationSettings)
	}
	if store.receiptContent != content {
		t.Fatalf("expected AI test not to change receipt content, got %#v", store.receiptContent)
	}
}

func TestSaveScheduleEndpointPersistsSettingsAndResetsScheduler(t *testing.T) {
	store := &fakeStore{}
	scheduler := &fakeScheduler{}
	server := NewServer(store, &fakePrinter{}, fixedClock, WithScheduler(scheduler))

	body := bytes.NewBufferString(`{"enabled":true,"mode":"daily_times","intervalMinutes":0,"times":["09:00","07:00","09:00"],"timezone":""}`)
	request := httptest.NewRequest(http.MethodPost, "/api/settings/schedule", body)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if !scheduleSettingsEqual(store.scheduleSettings, schedule.Settings{
		Enabled:         true,
		Mode:            schedule.ModeDailyTimes,
		IntervalMinutes: schedule.DefaultIntervalMinutes,
		Times:           []string{"07:00", "09:00"},
		Timezone:        schedule.DefaultTimezone,
	}) {
		t.Fatalf("expected normalized schedule, got %#v", store.scheduleSettings)
	}
	if scheduler.resetCalls != 1 {
		t.Fatalf("expected scheduler reset call, got %d", scheduler.resetCalls)
	}
}

func TestSaveScheduleEndpointRejectsInvalidSettings(t *testing.T) {
	store := &fakeStore{}
	scheduler := &fakeScheduler{}
	server := NewServer(store, &fakePrinter{}, fixedClock, WithScheduler(scheduler))

	body := bytes.NewBufferString(`{"enabled":true,"mode":"interval","intervalMinutes":2,"times":["07:00"],"timezone":"Europe/Minsk"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/settings/schedule", body)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
	}
	if scheduler.resetCalls != 0 {
		t.Fatalf("expected no scheduler reset, got %d", scheduler.resetCalls)
	}
}

func TestSchedulerStatusEndpointReturnsStatus(t *testing.T) {
	nextRun := time.Date(2026, 5, 25, 7, 0, 0, 0, time.UTC)
	scheduler := &fakeScheduler{status: schedulerruntime.Status{
		Settings: schedule.Settings{
			Enabled:         true,
			Mode:            schedule.ModeInterval,
			IntervalMinutes: 15,
			Timezone:        schedule.DefaultTimezone,
		},
		NextRunAt: nextRun,
	}}
	server := NewServer(&fakeStore{}, &fakePrinter{}, fixedClock, WithScheduler(scheduler))

	request := httptest.NewRequest(http.MethodGet, "/api/scheduler/status", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var payload schedulerStatusResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.OK || payload.Status == nil {
		t.Fatalf("expected scheduler status payload, got %#v", payload)
	}
	if !payload.Status.NextRunAt.Equal(nextRun) {
		t.Fatalf("expected next run %s, got %s", nextRun, payload.Status.NextRunAt)
	}
}

func TestStopScheduleEndpointDisablesScheduleAndResetsScheduler(t *testing.T) {
	store := &fakeStore{
		scheduleSettings: schedule.Settings{
			Enabled:         true,
			Mode:            schedule.ModeDailyTimes,
			IntervalMinutes: 15,
			Times:           []string{"07:00", "09:00"},
			Timezone:        schedule.DefaultTimezone,
		},
	}
	scheduler := &fakeScheduler{}
	server := NewServer(store, &fakePrinter{}, fixedClock, WithScheduler(scheduler))

	request := httptest.NewRequest(http.MethodPost, "/api/settings/schedule/stop", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if store.scheduleSettings.Enabled {
		t.Fatalf("expected schedule to be disabled, got %#v", store.scheduleSettings)
	}
	if scheduler.resetCalls != 1 {
		t.Fatalf("expected scheduler reset call, got %d", scheduler.resetCalls)
	}
}

func fixedClock() time.Time {
	return time.Date(2026, 5, 25, 9, 7, 0, 0, time.UTC)
}

type fakeStore struct {
	config             printer.Config
	location           weather.Location
	portfolio          finance.TonPortfolio
	newsSettings       news.Settings
	receiptStyle       receipt.StyleSettings
	receiptContent     receipt.ContentSettings
	scheduleSettings   schedule.Settings
	scheduleState      schedule.State
	motivationSettings motivation.Settings
}

func (s *fakeStore) LoadPrinter() (printer.Config, error) {
	return s.config, nil
}

func (s *fakeStore) SavePrinter(config printer.Config) error {
	normalized := config.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}
	s.config = normalized
	return nil
}

func (s *fakeStore) LoadWeather() (weather.Location, error) {
	if s.location == (weather.Location{}) {
		return weather.DefaultLocation(), nil
	}
	return s.location, nil
}

func (s *fakeStore) SaveWeather(location weather.Location) error {
	normalized := location.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}
	s.location = normalized
	return nil
}

func (s *fakeStore) LoadFinance() (finance.TonPortfolio, error) {
	if s.portfolio == (finance.TonPortfolio{}) {
		return finance.DefaultTonPortfolio(), nil
	}
	return s.portfolio, nil
}

func (s *fakeStore) SaveFinance(portfolio finance.TonPortfolio) error {
	normalized := portfolio.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}
	s.portfolio = normalized
	return nil
}

func (s *fakeStore) LoadNews() (news.Settings, error) {
	if len(s.newsSettings.Sources) == 0 {
		return news.DefaultSettings(), nil
	}
	return s.newsSettings.Normalized(), nil
}

func (s *fakeStore) SaveNews(settings news.Settings) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	s.newsSettings = settings.Normalized()
	return nil
}

func (s *fakeStore) LoadMotivation() (motivation.Settings, error) {
	if s.motivationSettings == (motivation.Settings{}) {
		return motivation.DefaultSettings(), nil
	}
	return s.motivationSettings.Normalized(), nil
}

func (s *fakeStore) SaveMotivation(settings motivation.Settings) error {
	normalized := settings.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}
	s.motivationSettings = normalized
	return nil
}

func (s *fakeStore) LoadReceiptStyle() (receipt.StyleSettings, error) {
	if s.receiptStyle == (receipt.StyleSettings{}) {
		return receipt.DefaultStyleSettings(), nil
	}
	return s.receiptStyle.Normalized(), nil
}

func (s *fakeStore) SaveReceiptStyle(style receipt.StyleSettings) error {
	s.receiptStyle = style.Normalized()
	return nil
}

func (s *fakeStore) LoadReceiptContent() (receipt.ContentSettings, error) {
	return s.receiptContent.Normalized(), nil
}

func (s *fakeStore) SaveReceiptContent(content receipt.ContentSettings) error {
	s.receiptContent = content.Normalized()
	return nil
}

func (s *fakeStore) LoadSchedule() (schedule.Settings, error) {
	if scheduleSettingsEqual(s.scheduleSettings, schedule.Settings{}) {
		return schedule.DefaultSettings(), nil
	}
	return s.scheduleSettings.Normalized(), nil
}

func (s *fakeStore) SaveSchedule(settings schedule.Settings) error {
	normalized := settings.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}
	s.scheduleSettings = normalized
	return nil
}

func (s *fakeStore) LoadScheduleState() (schedule.State, error) {
	return s.scheduleState, nil
}

func (s *fakeStore) SaveScheduleState(state schedule.State) error {
	s.scheduleState = state
	return nil
}

type fakeScheduler struct {
	resetCalls int
	status     schedulerruntime.Status
}

func (s *fakeScheduler) ResetFromNow(context.Context) error {
	s.resetCalls++
	return nil
}

func (s *fakeScheduler) Status() (schedulerruntime.Status, error) {
	return s.status, nil
}

type fakePrinter struct {
	checkMessage             string
	checkedConfig            printer.Config
	fontMetricsConfig        printer.Config
	fontMetrics              []printer.FontMetric
	printedConfig            printer.Config
	printedLines             []receipt.Line
	printedPixelBufferConfig printer.Config
	printedPixelBuffer       printer.PixelBuffer
}

func (p *fakePrinter) CheckConnection(_ context.Context, config printer.Config) (string, error) {
	p.checkedConfig = config
	if p.checkMessage == "" {
		return "Подключено", nil
	}
	return p.checkMessage, nil
}

func (p *fakePrinter) PrintReceipt(_ context.Context, config printer.Config, lines []receipt.Line) error {
	p.printedConfig = config
	p.printedLines = append([]receipt.Line(nil), lines...)
	return nil
}

func (p *fakePrinter) PrintPixelBuffer(_ context.Context, config printer.Config, buffer printer.PixelBuffer) error {
	p.printedPixelBufferConfig = config
	p.printedPixelBuffer = buffer.Clone()
	return nil
}

func (p *fakePrinter) FontMetrics(_ context.Context, config printer.Config) ([]printer.FontMetric, error) {
	p.fontMetricsConfig = config
	if p.fontMetrics == nil {
		return []printer.FontMetric{{Font: 0, LineLength: 32, FontWidth: 12}}, nil
	}
	return append([]printer.FontMetric(nil), p.fontMetrics...), nil
}

func (p *fakePrinter) DriverVersion() (string, error) {
	return "10.10.8.0", nil
}

type fakeWeatherProvider struct {
	location weather.Location
	snapshot weather.Snapshot
}

func (p *fakeWeatherProvider) Current(_ context.Context, location weather.Location) (weather.Snapshot, error) {
	p.location = location
	return p.snapshot, nil
}

type fakeLocationSearchProvider struct {
	query   string
	results []weather.LocationCandidate
}

func (p *fakeLocationSearchProvider) Search(_ context.Context, query string) ([]weather.LocationCandidate, error) {
	p.query = query
	return p.results, nil
}

type fakeTonProvider struct {
	price finance.TonPrice
}

func (p *fakeTonProvider) CurrentPrice(context.Context) (finance.TonPrice, error) {
	return p.price, nil
}

type fakeFiatProvider struct {
	rate finance.FiatRate
}

func (p *fakeFiatProvider) CurrentRate(context.Context) (finance.FiatRate, error) {
	return p.rate, nil
}

type fakeNewsProvider struct {
	settings news.Settings
	items    []news.Item
}

func (p *fakeNewsProvider) Current(_ context.Context, settings news.Settings) ([]news.Item, error) {
	p.settings = settings
	return p.items, nil
}

type fakeGoogleClient struct {
	status              googleintegration.Status
	authURL             string
	authRedirectURI     string
	authState           string
	exchangedCode       string
	exchangeRedirectURI string
	disconnected        bool
	summary             googleintegration.Summary
	err                 error
}

func (c *fakeGoogleClient) Current(context.Context) (googleintegration.Summary, error) {
	return c.summary, c.err
}

func (c *fakeGoogleClient) Status() googleintegration.Status {
	return c.status
}

func (c *fakeGoogleClient) AuthURL(redirectURI string, state string) (string, error) {
	c.authRedirectURI = redirectURI
	c.authState = state
	if c.err != nil {
		return "", c.err
	}
	if c.authURL == "" {
		return "https://accounts.example/auth", nil
	}
	return c.authURL, nil
}

func (c *fakeGoogleClient) ExchangeCode(_ context.Context, code string, redirectURI string) error {
	c.exchangedCode = code
	c.exchangeRedirectURI = redirectURI
	return c.err
}

func (c *fakeGoogleClient) Disconnect() error {
	c.disconnected = true
	return c.err
}

type fakeMotivationProvider struct {
	quote  motivation.Quote
	advice motivation.WeatherAdvice
	err    error
}

func (p *fakeMotivationProvider) Generate(context.Context, motivation.Settings) (motivation.Quote, error) {
	return p.quote, p.err
}

func (p *fakeMotivationProvider) GenerateWeatherAdvice(context.Context, motivation.Settings, motivation.WeatherContext) (motivation.WeatherAdvice, error) {
	return p.advice, p.err
}

func (p *fakeMotivationProvider) TranslateNewsTitles(context.Context, motivation.Settings, []motivation.NewsTitle) ([]motivation.NewsTranslation, error) {
	return nil, p.err
}

func lineTextsContain(lines []receipt.Line, want string) bool {
	for _, line := range lines {
		if line.Text == want {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func scheduleSettingsEqual(left schedule.Settings, right schedule.Settings) bool {
	if left.Enabled != right.Enabled ||
		left.Mode != right.Mode ||
		left.IntervalMinutes != right.IntervalMinutes ||
		left.Timezone != right.Timezone ||
		len(left.Times) != len(right.Times) {
		return false
	}
	for index := range left.Times {
		if left.Times[index] != right.Times[index] {
			return false
		}
	}
	return true
}

func imageEditorSaveJSON(t *testing.T, width int, height int, mutate func([]int) []int) []byte {
	t.Helper()
	pixels := make([]int, width*height)
	for index := range pixels {
		if index%2 == 1 {
			pixels[index] = 255
		}
	}
	if mutate != nil {
		pixels = mutate(pixels)
	}
	payload := map[string]any{
		"width":      width,
		"height":     height,
		"pixels":     pixels,
		"previewPng": tinyPNGDataURL,
		"settings": map[string]any{
			"threshold": 128,
			"dither":    true,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal image editor payload: %v", err)
	}
	return data
}

const tinyPNGDataURL = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lJypqAAAAABJRU5ErkJggg=="
