package googleintegration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestClientBuildsAuthURLFromCredentials(t *testing.T) {
	dir := t.TempDir()
	credentialsPath := filepath.Join(dir, "credentials.json")
	writeCredentials(t, credentialsPath, "https://accounts.example/auth", "https://accounts.example/token")

	client := NewClient(Config{
		CredentialsPath: credentialsPath,
		TokenPath:       filepath.Join(dir, "token.json"),
	})

	authURL, err := client.AuthURL("http://localhost:8080/oauth/google/callback", "state-123")
	if err != nil {
		t.Fatalf("build auth URL: %v", err)
	}

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	query := parsed.Query()
	if parsed.String() == "" || parsed.Scheme != "https" || parsed.Host != "accounts.example" || parsed.Path != "/auth" {
		t.Fatalf("unexpected auth URL: %s", authURL)
	}
	if query.Get("client_id") != "client-id" {
		t.Fatalf("expected client id, got %q", query.Get("client_id"))
	}
	if query.Get("redirect_uri") != "http://localhost:8080/oauth/google/callback" {
		t.Fatalf("expected redirect URI, got %q", query.Get("redirect_uri"))
	}
	if query.Get("state") != "state-123" || query.Get("access_type") != "offline" || query.Get("prompt") != "consent" {
		t.Fatalf("expected offline consent state params, got %s", query.Encode())
	}
	scope := query.Get("scope")
	for _, want := range []string{ScopeGmailReadonly, ScopeCalendarReadonly} {
		if !strings.Contains(scope, want) {
			t.Fatalf("expected scope %q in %q", want, scope)
		}
	}
}

func TestClientExchangeCodeSavesToken(t *testing.T) {
	dir := t.TempDir()
	var form url.Values
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse token form: %v", err)
		}
		form = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-1","refresh_token":"refresh-1","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	credentialsPath := filepath.Join(dir, "credentials.json")
	tokenPath := filepath.Join(dir, "token.json")
	writeCredentials(t, credentialsPath, "https://accounts.example/auth", tokenServer.URL)
	client := NewClient(Config{
		CredentialsPath: credentialsPath,
		TokenPath:       tokenPath,
		Clock:           func() time.Time { return time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC) },
	})

	if err := client.ExchangeCode(context.Background(), "auth-code", "http://localhost:8080/oauth/google/callback"); err != nil {
		t.Fatalf("exchange code: %v", err)
	}

	if form.Get("grant_type") != "authorization_code" || form.Get("code") != "auth-code" || form.Get("client_id") != "client-id" || form.Get("client_secret") != "client-secret" {
		t.Fatalf("unexpected token form: %s", form.Encode())
	}
	saved, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("read saved token: %v", err)
	}
	var token Token
	if err := json.Unmarshal(saved, &token); err != nil {
		t.Fatalf("decode saved token: %v", err)
	}
	if token.AccessToken != "access-1" || token.RefreshToken != "refresh-1" || !token.Expiry.After(time.Date(2026, 5, 25, 10, 59, 0, 0, time.UTC)) {
		t.Fatalf("unexpected saved token: %#v", token)
	}
}

func TestClientExchangeCodeSavesTokenToConfiguredStore(t *testing.T) {
	dir := t.TempDir()
	var form url.Values
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse token form: %v", err)
		}
		form = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-db","refresh_token":"refresh-db","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	credentialsPath := filepath.Join(dir, "credentials.json")
	writeCredentials(t, credentialsPath, "https://accounts.example/auth", tokenServer.URL)
	tokenStore := &memoryTokenStore{}
	client := NewClient(Config{
		CredentialsPath: credentialsPath,
		TokenStore:      tokenStore,
		Clock:           func() time.Time { return time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC) },
	})

	if err := client.ExchangeCode(context.Background(), "auth-code", "http://localhost:8080/oauth/google/callback"); err != nil {
		t.Fatalf("exchange code: %v", err)
	}

	if form.Get("grant_type") != "authorization_code" || form.Get("code") != "auth-code" {
		t.Fatalf("unexpected token form: %s", form.Encode())
	}
	if tokenStore.saved.AccessToken != "access-db" || tokenStore.saved.RefreshToken != "refresh-db" {
		t.Fatalf("expected token saved to store, got %#v", tokenStore.saved)
	}
	if status := client.Status(); !status.TokenAvailable || !status.Authorized {
		t.Fatalf("expected token store to make client authorized, got %#v", status)
	}
}

func TestClientExchangeCodePreservesExistingRefreshTokenWhenResponseOmitsIt(t *testing.T) {
	dir := t.TempDir()
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-new","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	credentialsPath := filepath.Join(dir, "credentials.json")
	writeCredentials(t, credentialsPath, "https://accounts.example/auth", tokenServer.URL)
	tokenStore := &memoryTokenStore{saved: Token{
		AccessToken:  "access-old",
		RefreshToken: "refresh-old",
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC),
	}}
	client := NewClient(Config{
		CredentialsPath: credentialsPath,
		TokenStore:      tokenStore,
		Clock:           func() time.Time { return time.Date(2026, 5, 25, 11, 0, 0, 0, time.UTC) },
	})

	if err := client.ExchangeCode(context.Background(), "auth-code", "http://localhost:8080/oauth/google/callback"); err != nil {
		t.Fatalf("exchange code: %v", err)
	}

	if tokenStore.saved.AccessToken != "access-new" || tokenStore.saved.RefreshToken != "refresh-old" {
		t.Fatalf("expected existing refresh token to be preserved, got %#v", tokenStore.saved)
	}
}

func TestClientCurrentReturnsTokenStoreErrors(t *testing.T) {
	dir := t.TempDir()
	credentialsPath := filepath.Join(dir, "credentials.json")
	writeCredentials(t, credentialsPath, "https://accounts.example/auth", "https://accounts.example/token")
	tokenStore := &memoryTokenStore{err: errors.New("db unavailable")}
	client := NewClient(Config{
		CredentialsPath: credentialsPath,
		TokenStore:      tokenStore,
	})

	_, err := client.Current(context.Background())

	if err == nil || !strings.Contains(err.Error(), "db unavailable") {
		t.Fatalf("expected token store error, got %v", err)
	}
}

func TestClientLoadsUnreadMailAndTodayCalendarEvents(t *testing.T) {
	dir := t.TempDir()
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer access-1" {
			t.Fatalf("expected bearer token, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/gmail/v1/users/me/messages":
			if r.URL.Query().Get("q") != "" || !reflect.DeepEqual(r.URL.Query()["labelIds"], []string{"INBOX", "UNREAD"}) || r.URL.Query().Get("maxResults") != "5" {
				t.Fatalf("unexpected gmail list query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"messages":[{"id":"m1"},{"id":"m2"}]}`))
		case r.URL.Path == "/gmail/v1/users/me/messages/m1":
			_, _ = w.Write([]byte(`{"labelIds":["INBOX","UNREAD"],"payload":{"headers":[{"name":"From","value":"Alice <alice@example.com>"},{"name":"Subject","value":"Morning update"}]}}`))
		case r.URL.Path == "/gmail/v1/users/me/messages/m2":
			_, _ = w.Write([]byte(`{"labelIds":["INBOX"],"payload":{"headers":[{"name":"From","value":"Bob <bob@example.com>"},{"name":"Subject","value":"Invoice"}]}}`))
		case r.URL.Path == "/calendar/v3/users/me/calendarList":
			if r.URL.Query().Get("minAccessRole") != "reader" {
				t.Fatalf("unexpected calendar list query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"items":[{"id":"personal@example.com","summary":"Personal","primary":true,"accessRole":"owner"},{"id":"work@example.com","summary":"Work","selected":true,"accessRole":"reader"},{"id":"availability@example.com","summary":"Availability","selected":true,"accessRole":"freeBusyReader"}]}`))
		case r.URL.Path == "/calendar/v3/calendars/personal@example.com/events":
			if r.URL.Query().Get("singleEvents") != "true" || r.URL.Query().Get("orderBy") != "startTime" {
				t.Fatalf("unexpected calendar query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"items":[{"summary":"Ветеринар","start":{"dateTime":"2026-05-25T18:30:00+03:00"}},{"summary":"День без встреч","start":{"date":"2026-05-25"}}]}`))
		case r.URL.Path == "/calendar/v3/calendars/work@example.com/events":
			if r.URL.Query().Get("singleEvents") != "true" || r.URL.Query().Get("orderBy") != "startTime" {
				t.Fatalf("unexpected shared calendar query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"items":[{"summary":"Рабочий стендап","start":{"dateTime":"2026-05-25T09:30:00+03:00"}}]}`))
		case r.URL.Path == "/calendar/v3/calendars/availability@example.com/events":
			t.Fatal("free/busy calendars must not be queried for event details")
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	credentialsPath := filepath.Join(dir, "credentials.json")
	tokenPath := filepath.Join(dir, "token.json")
	writeCredentials(t, credentialsPath, "https://accounts.example/auth", "https://accounts.example/token")
	writeToken(t, tokenPath, Token{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
	})
	client := NewClient(Config{
		CredentialsPath: credentialsPath,
		TokenPath:       tokenPath,
		GmailBaseURL:    apiServer.URL + "/gmail/v1",
		CalendarBaseURL: apiServer.URL + "/calendar/v3",
		Clock:           func() time.Time { return time.Date(2026, 5, 25, 9, 7, 0, 0, time.FixedZone("MSK", 3*60*60)) },
	})

	summary, err := client.Current(context.Background())
	if err != nil {
		t.Fatalf("load current google summary: %v", err)
	}
	if len(summary.Mail) != 1 || summary.Mail[0].From != "Alice" || summary.Mail[0].Subject != "Morning update" {
		t.Fatalf("unexpected mail summary: %#v", summary.Mail)
	}
	if len(summary.Events) != 3 ||
		summary.Events[0].TimeLabel != "Весь день" ||
		summary.Events[0].Title != "День без встреч" ||
		summary.Events[1].TimeLabel != "09:30" ||
		summary.Events[1].Title != "Рабочий стендап" ||
		summary.Events[2].TimeLabel != "18:30" ||
		summary.Events[2].Title != "Ветеринар" {
		t.Fatalf("unexpected calendar summary: %#v", summary.Events)
	}
}

func TestClientCurrentSelectedLoadsOnlyRequestedSections(t *testing.T) {
	dir := t.TempDir()
	var gmailHits int
	var calendarHits int
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer access-1" {
			t.Fatalf("expected bearer token, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/gmail/v1/"):
			gmailHits++
			switch r.URL.Path {
			case "/gmail/v1/users/me/messages":
				_, _ = w.Write([]byte(`{"messages":[{"id":"m1"}]}`))
			case "/gmail/v1/users/me/messages/m1":
				_, _ = w.Write([]byte(`{"labelIds":["INBOX","UNREAD"],"payload":{"headers":[{"name":"From","value":"Alice <alice@example.com>"},{"name":"Subject","value":"Morning update"}]}}`))
			default:
				t.Fatalf("unexpected Gmail path: %s", r.URL.Path)
			}
		case strings.HasPrefix(r.URL.Path, "/calendar/v3/"):
			calendarHits++
			switch r.URL.Path {
			case "/calendar/v3/users/me/calendarList":
				_, _ = w.Write([]byte(`{"items":[{"id":"work@example.com","summary":"Work","selected":true,"accessRole":"reader"}]}`))
			case "/calendar/v3/calendars/work@example.com/events":
				_, _ = w.Write([]byte(`{"items":[{"summary":"Рабочий стендап","start":{"dateTime":"2026-05-25T09:30:00+03:00"}}]}`))
			default:
				t.Fatalf("unexpected Calendar path: %s", r.URL.Path)
			}
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	credentialsPath := filepath.Join(dir, "credentials.json")
	tokenPath := filepath.Join(dir, "token.json")
	writeCredentials(t, credentialsPath, "https://accounts.example/auth", "https://accounts.example/token")
	writeToken(t, tokenPath, Token{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
	})
	client := NewClient(Config{
		CredentialsPath: credentialsPath,
		TokenPath:       tokenPath,
		GmailBaseURL:    apiServer.URL + "/gmail/v1",
		CalendarBaseURL: apiServer.URL + "/calendar/v3",
		Clock:           func() time.Time { return time.Date(2026, 5, 25, 9, 7, 0, 0, time.FixedZone("MSK", 3*60*60)) },
	})

	summary, err := client.CurrentSelected(context.Background(), false, true)
	if err != nil {
		t.Fatalf("load calendar-only summary: %v", err)
	}
	if gmailHits != 0 {
		t.Fatalf("Gmail must not be queried when mail is disabled, got %d calls", gmailHits)
	}
	if calendarHits == 0 || len(summary.Events) != 1 || len(summary.Mail) != 0 {
		t.Fatalf("expected calendar-only summary, hits=%d summary=%#v", calendarHits, summary)
	}

	gmailHits = 0
	calendarHits = 0
	summary, err = client.CurrentSelected(context.Background(), true, false)
	if err != nil {
		t.Fatalf("load mail-only summary: %v", err)
	}
	if calendarHits != 0 {
		t.Fatalf("Calendar must not be queried when calendar is disabled, got %d calls", calendarHits)
	}
	if gmailHits == 0 || len(summary.Mail) != 1 || len(summary.Events) != 0 {
		t.Fatalf("expected mail-only summary, hits=%d summary=%#v", gmailHits, summary)
	}
}

func TestClientLoadsTodayAndTomorrowEventsAfterCalendarSplitTime(t *testing.T) {
	dir := t.TempDir()
	var eventQueries []url.Values
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer access-1" {
			t.Fatalf("expected bearer token, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/calendar/v3/users/me/calendarList":
			_, _ = w.Write([]byte(`{"items":[{"id":"work@example.com","summary":"Work","selected":true,"accessRole":"reader"}]}`))
		case "/calendar/v3/calendars/work@example.com/events":
			eventQueries = append(eventQueries, r.URL.Query())
			_, _ = w.Write([]byte(`{"items":[
				{"summary":"Утренний созвон","start":{"dateTime":"2026-05-25T09:00:00+03:00"},"end":{"dateTime":"2026-05-25T09:30:00+03:00"}},
				{"summary":"Текущая встреча","start":{"dateTime":"2026-05-25T14:30:00+03:00"},"end":{"dateTime":"2026-05-25T15:30:00+03:00"}},
				{"summary":"День без встреч","start":{"date":"2026-05-25"},"end":{"date":"2026-05-26"}},
				{"summary":"Завтрашний план","start":{"dateTime":"2026-05-26T10:00:00+03:00"},"end":{"dateTime":"2026-05-26T11:00:00+03:00"}}
			]}`))
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	credentialsPath := filepath.Join(dir, "credentials.json")
	tokenPath := filepath.Join(dir, "token.json")
	writeCredentials(t, credentialsPath, "https://accounts.example/auth", "https://accounts.example/token")
	writeToken(t, tokenPath, Token{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 5, 25, 18, 0, 0, 0, time.UTC),
	})
	client := NewClient(Config{
		CredentialsPath: credentialsPath,
		TokenPath:       tokenPath,
		CalendarBaseURL: apiServer.URL + "/calendar/v3",
		Clock:           func() time.Time { return time.Date(2026, 5, 25, 15, 0, 0, 0, time.FixedZone("MSK", 3*60*60)) },
	})

	summary, err := client.CurrentSelected(context.Background(), false, true)
	if err != nil {
		t.Fatalf("load calendar summary: %v", err)
	}
	if len(eventQueries) != 1 {
		t.Fatalf("expected one event query, got %d", len(eventQueries))
	}
	if got := eventQueries[0].Get("timeMin"); got != "2026-05-25T00:00:00+03:00" {
		t.Fatalf("unexpected timeMin: %s", got)
	}
	if got := eventQueries[0].Get("timeMax"); got != "2026-05-27T00:00:00+03:00" {
		t.Fatalf("unexpected timeMax: %s", got)
	}
	if got := eventQueries[0].Get("maxResults"); got != "20" {
		t.Fatalf("unexpected maxResults for two-day calendar window: %s", got)
	}
	if len(summary.Events) != 3 ||
		summary.Events[0].Title != "День без встреч" ||
		summary.Events[0].TimeLabel != "Весь день" ||
		!summary.Events[0].AllDay ||
		summary.Events[1].Title != "Текущая встреча" ||
		summary.Events[1].TimeLabel != "14:30" ||
		summary.Events[2].Title != "Завтрашний план" ||
		summary.Events[2].Start.Day() != 26 {
		t.Fatalf("unexpected calendar events after split time: %#v", summary.Events)
	}
}

func TestClientLoadsOnlyTodayBeforeCalendarSplitTime(t *testing.T) {
	dir := t.TempDir()
	var eventQuery url.Values
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer access-1" {
			t.Fatalf("expected bearer token, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/calendar/v3/users/me/calendarList":
			_, _ = w.Write([]byte(`{"items":[{"id":"work@example.com","summary":"Work","selected":true,"accessRole":"reader"}]}`))
		case "/calendar/v3/calendars/work@example.com/events":
			eventQuery = r.URL.Query()
			_, _ = w.Write([]byte(`{"items":[
				{"summary":"Утро","start":{"dateTime":"2026-05-25T09:00:00+03:00"},"end":{"dateTime":"2026-05-25T09:30:00+03:00"}},
				{"summary":"Вечер","start":{"dateTime":"2026-05-25T18:00:00+03:00"},"end":{"dateTime":"2026-05-25T19:00:00+03:00"}}
			]}`))
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	credentialsPath := filepath.Join(dir, "credentials.json")
	tokenPath := filepath.Join(dir, "token.json")
	writeCredentials(t, credentialsPath, "https://accounts.example/auth", "https://accounts.example/token")
	writeToken(t, tokenPath, Token{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 5, 25, 18, 0, 0, 0, time.UTC),
	})
	client := NewClient(Config{
		CredentialsPath: credentialsPath,
		TokenPath:       tokenPath,
		CalendarBaseURL: apiServer.URL + "/calendar/v3",
		Clock:           func() time.Time { return time.Date(2026, 5, 25, 14, 59, 0, 0, time.FixedZone("MSK", 3*60*60)) },
	})

	summary, err := client.CurrentSelected(context.Background(), false, true)
	if err != nil {
		t.Fatalf("load calendar summary: %v", err)
	}
	if got := eventQuery.Get("timeMin"); got != "2026-05-25T00:00:00+03:00" {
		t.Fatalf("unexpected timeMin: %s", got)
	}
	if got := eventQuery.Get("timeMax"); got != "2026-05-26T00:00:00+03:00" {
		t.Fatalf("unexpected timeMax: %s", got)
	}
	if got := eventQuery.Get("maxResults"); got != "10" {
		t.Fatalf("unexpected maxResults for one-day calendar window: %s", got)
	}
	if len(summary.Events) != 2 || summary.Events[0].Title != "Утро" || summary.Events[1].Title != "Вечер" {
		t.Fatalf("expected all today events before split time, got %#v", summary.Events)
	}
}

func writeCredentials(t *testing.T, path string, authURI string, tokenURI string) {
	t.Helper()
	data := map[string]any{
		"installed": map[string]any{
			"client_id":     "client-id",
			"client_secret": "client-secret",
			"auth_uri":      authURI,
			"token_uri":     tokenURI,
		},
	}
	body, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal credentials: %v", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}
}

func writeToken(t *testing.T, path string, token Token) {
	t.Helper()
	body, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("marshal token: %v", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write token: %v", err)
	}
}

type memoryTokenStore struct {
	saved   Token
	deleted bool
	err     error
}

func (s *memoryTokenStore) LoadToken(context.Context) (Token, error) {
	if s.err != nil {
		return Token{}, s.err
	}
	if s.deleted || (s.saved.AccessToken == "" && s.saved.RefreshToken == "") {
		return Token{}, ErrNotAuthorized
	}
	return s.saved, nil
}

func (s *memoryTokenStore) SaveToken(_ context.Context, token Token) error {
	s.saved = token
	s.deleted = false
	return nil
}

func (s *memoryTokenStore) DeleteToken(context.Context) error {
	s.saved = Token{}
	s.deleted = true
	return nil
}
