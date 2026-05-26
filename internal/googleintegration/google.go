package googleintegration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	ScopeGmailReadonly    = "https://www.googleapis.com/auth/gmail.readonly"
	ScopeCalendarReadonly = "https://www.googleapis.com/auth/calendar.readonly"

	defaultAuthURI       = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenURI      = "https://oauth2.googleapis.com/token"
	defaultGmailBaseURL  = "https://gmail.googleapis.com/gmail/v1"
	defaultCalendarURL   = "https://www.googleapis.com/calendar/v3"
	defaultMaxMail       = 5
	defaultMaxEvents     = 10
	googleRequestTimeout = 20 * time.Second
)

var (
	ErrNotConfigured = errors.New("google credentials are not configured")
	ErrNotAuthorized = errors.New("google token is not configured")
)

type Config struct {
	CredentialsPath string
	TokenPath       string
	GmailBaseURL    string
	CalendarBaseURL string
	Client          *http.Client
	Clock           func() time.Time
}

type Client struct {
	config Config
}

type Status struct {
	CredentialsAvailable bool   `json:"credentialsAvailable"`
	TokenAvailable       bool   `json:"tokenAvailable"`
	Authorized           bool   `json:"authorized"`
	CredentialsPath      string `json:"credentialsPath"`
	TokenPath            string `json:"tokenPath"`
}

type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
}

type Summary struct {
	Mail   []MailMessage   `json:"mail"`
	Events []CalendarEvent `json:"events"`
}

type MailMessage struct {
	From    string
	Subject string
}

type CalendarEvent struct {
	TimeLabel string
	Title     string
}

type calendarListEntry struct {
	ID         string `json:"id"`
	Summary    string `json:"summary"`
	AccessRole string `json:"accessRole"`
	Primary    bool   `json:"primary"`
	Selected   bool   `json:"selected"`
	Hidden     bool   `json:"hidden"`
	Deleted    bool   `json:"deleted"`
}

type calendarEventResult struct {
	event  CalendarEvent
	sortAt time.Time
}

type credentialsFile struct {
	Installed *oauthClient `json:"installed"`
	Web       *oauthClient `json:"web"`
}

type oauthClient struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	AuthURI      string `json:"auth_uri"`
	TokenURI     string `json:"token_uri"`
}

func NewClient(config Config) *Client {
	if config.Client == nil {
		config.Client = &http.Client{Timeout: googleRequestTimeout}
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.GmailBaseURL == "" {
		config.GmailBaseURL = defaultGmailBaseURL
	}
	if config.CalendarBaseURL == "" {
		config.CalendarBaseURL = defaultCalendarURL
	}
	return &Client{config: config}
}

func DefaultConfig(dataDir string) Config {
	return Config{
		CredentialsPath: filepath.Join(dataDir, "google", "credentials.json"),
		TokenPath:       filepath.Join(dataDir, "google", "token.json"),
	}
}

func (c *Client) Status() Status {
	credentialsAvailable := fileExists(c.config.CredentialsPath)
	tokenAvailable := fileExists(c.config.TokenPath)
	return Status{
		CredentialsAvailable: credentialsAvailable,
		TokenAvailable:       tokenAvailable,
		Authorized:           credentialsAvailable && tokenAvailable,
		CredentialsPath:      c.config.CredentialsPath,
		TokenPath:            c.config.TokenPath,
	}
}

func (c *Client) AuthURL(redirectURI string, state string) (string, error) {
	credentials, err := c.loadCredentials()
	if err != nil {
		return "", err
	}
	authURI := strings.TrimSpace(credentials.AuthURI)
	if authURI == "" {
		authURI = defaultAuthURI
	}
	values := url.Values{}
	values.Set("client_id", credentials.ClientID)
	values.Set("redirect_uri", redirectURI)
	values.Set("response_type", "code")
	values.Set("scope", strings.Join([]string{ScopeGmailReadonly, ScopeCalendarReadonly}, " "))
	values.Set("access_type", "offline")
	values.Set("prompt", "consent")
	values.Set("state", state)
	return authURI + "?" + values.Encode(), nil
}

func (c *Client) ExchangeCode(ctx context.Context, code string, redirectURI string) error {
	credentials, err := c.loadCredentials()
	if err != nil {
		return err
	}
	tokenURI := strings.TrimSpace(credentials.TokenURI)
	if tokenURI == "" {
		tokenURI = defaultTokenURI
	}

	values := url.Values{}
	values.Set("client_id", credentials.ClientID)
	values.Set("client_secret", credentials.ClientSecret)
	values.Set("code", code)
	values.Set("redirect_uri", redirectURI)
	values.Set("grant_type", "authorization_code")

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	response, err := c.client().Do(request)
	if err != nil {
		return fmt.Errorf("exchange Google OAuth code: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("exchange Google OAuth code returned HTTP %d", response.StatusCode)
	}

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return fmt.Errorf("decode Google OAuth token: %w", err)
	}
	token := Token{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		TokenType:    payload.TokenType,
		Expiry:       c.now().Add(time.Duration(payload.ExpiresIn) * time.Second),
	}
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}
	if token.AccessToken == "" {
		return fmt.Errorf("Google OAuth token response has no access token")
	}
	return c.saveToken(token)
}

func (c *Client) Disconnect() error {
	if c.config.TokenPath == "" {
		return nil
	}
	if err := os.Remove(c.config.TokenPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (c *Client) Current(ctx context.Context) (Summary, error) {
	return c.CurrentSelected(ctx, true, true)
}

func (c *Client) CurrentSelected(ctx context.Context, includeMail bool, includeCalendar bool) (Summary, error) {
	if !includeMail && !includeCalendar {
		return Summary{}, nil
	}
	if !fileExists(c.config.CredentialsPath) {
		return Summary{}, nil
	}
	if !fileExists(c.config.TokenPath) {
		return Summary{}, nil
	}
	token, err := c.validToken(ctx)
	if err != nil {
		return Summary{}, err
	}
	var summary Summary
	if includeMail {
		mail, err := c.unreadMail(ctx, token)
		if err != nil {
			return Summary{}, err
		}
		summary.Mail = mail
	}
	if includeCalendar {
		events, err := c.todayEvents(ctx, token)
		if err != nil {
			return Summary{}, err
		}
		summary.Events = events
	}
	return summary, nil
}

func (c *Client) validToken(ctx context.Context) (Token, error) {
	token, err := c.loadToken()
	if err != nil {
		return Token{}, err
	}
	if token.AccessToken != "" && token.Expiry.After(c.now().Add(time.Minute)) {
		return token, nil
	}
	if token.RefreshToken == "" {
		return Token{}, ErrNotAuthorized
	}
	refreshed, err := c.refreshToken(ctx, token.RefreshToken)
	if err != nil {
		return Token{}, err
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = token.RefreshToken
	}
	if err := c.saveToken(refreshed); err != nil {
		return Token{}, err
	}
	return refreshed, nil
}

func (c *Client) refreshToken(ctx context.Context, refreshToken string) (Token, error) {
	credentials, err := c.loadCredentials()
	if err != nil {
		return Token{}, err
	}
	tokenURI := strings.TrimSpace(credentials.TokenURI)
	if tokenURI == "" {
		tokenURI = defaultTokenURI
	}
	values := url.Values{}
	values.Set("client_id", credentials.ClientID)
	values.Set("client_secret", credentials.ClientSecret)
	values.Set("refresh_token", refreshToken)
	values.Set("grant_type", "refresh_token")

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI, strings.NewReader(values.Encode()))
	if err != nil {
		return Token{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := c.client().Do(request)
	if err != nil {
		return Token{}, fmt.Errorf("refresh Google OAuth token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return Token{}, fmt.Errorf("refresh Google OAuth token returned HTTP %d", response.StatusCode)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return Token{}, fmt.Errorf("decode refreshed Google OAuth token: %w", err)
	}
	if payload.AccessToken == "" {
		return Token{}, fmt.Errorf("refreshed Google OAuth token has no access token")
	}
	if payload.TokenType == "" {
		payload.TokenType = "Bearer"
	}
	return Token{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		TokenType:    payload.TokenType,
		Expiry:       c.now().Add(time.Duration(payload.ExpiresIn) * time.Second),
	}, nil
}

func (c *Client) unreadMail(ctx context.Context, token Token) ([]MailMessage, error) {
	baseURL := strings.TrimRight(c.config.GmailBaseURL, "/")
	values := url.Values{}
	values.Set("q", "is:unread")
	values.Set("maxResults", fmt.Sprintf("%d", defaultMaxMail))
	listURL := baseURL + "/users/me/messages?" + values.Encode()

	var list struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := c.getJSON(ctx, token, listURL, &list); err != nil {
		return nil, fmt.Errorf("load Gmail unread messages: %w", err)
	}

	messages := make([]MailMessage, 0, len(list.Messages))
	for _, item := range list.Messages {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		detailValues := url.Values{}
		detailValues.Set("format", "metadata")
		detailValues.Add("metadataHeaders", "From")
		detailValues.Add("metadataHeaders", "Subject")
		detailURL := baseURL + "/users/me/messages/" + url.PathEscape(item.ID) + "?" + detailValues.Encode()
		var detail struct {
			Payload struct {
				Headers []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"headers"`
			} `json:"payload"`
		}
		if err := c.getJSON(ctx, token, detailURL, &detail); err != nil {
			return nil, fmt.Errorf("load Gmail message %s: %w", item.ID, err)
		}
		message := MailMessage{
			From:    cleanSender(headerValue(detail.Payload.Headers, "From")),
			Subject: strings.TrimSpace(headerValue(detail.Payload.Headers, "Subject")),
		}
		if message.From == "" && message.Subject == "" {
			continue
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func (c *Client) todayEvents(ctx context.Context, token Token) ([]CalendarEvent, error) {
	location := minskLocation()
	now := c.now().In(location)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	end := start.AddDate(0, 0, 1)

	calendars, err := c.calendarList(ctx, token)
	if err != nil {
		return nil, err
	}
	if len(calendars) == 0 {
		calendars = []calendarListEntry{{ID: "primary", Primary: true, AccessRole: "reader"}}
	}

	var allEvents []calendarEventResult
	for _, calendar := range calendars {
		if !calendar.canReadEvents() {
			continue
		}
		events, err := c.eventsForCalendar(ctx, token, calendar.ID, location, start, end)
		if err != nil {
			return nil, err
		}
		allEvents = append(allEvents, events...)
	}

	sort.SliceStable(allEvents, func(i, j int) bool {
		if allEvents[i].sortAt.Equal(allEvents[j].sortAt) {
			return allEvents[i].event.Title < allEvents[j].event.Title
		}
		return allEvents[i].sortAt.Before(allEvents[j].sortAt)
	})
	if len(allEvents) > defaultMaxEvents {
		allEvents = allEvents[:defaultMaxEvents]
	}

	events := make([]CalendarEvent, 0, len(allEvents))
	for _, item := range allEvents {
		events = append(events, item.event)
	}
	return events, nil
}

func (c *Client) calendarList(ctx context.Context, token Token) ([]calendarListEntry, error) {
	baseURL := strings.TrimRight(c.config.CalendarBaseURL, "/")
	values := url.Values{}
	values.Set("minAccessRole", "reader")
	requestURL := baseURL + "/users/me/calendarList?" + values.Encode()

	var payload struct {
		Items []calendarListEntry `json:"items"`
	}
	if err := c.getJSON(ctx, token, requestURL, &payload); err != nil {
		return nil, fmt.Errorf("load Google Calendar list: %w", err)
	}
	return payload.Items, nil
}

func (c *Client) eventsForCalendar(ctx context.Context, token Token, calendarID string, location *time.Location, start time.Time, end time.Time) ([]calendarEventResult, error) {
	baseURL := strings.TrimRight(c.config.CalendarBaseURL, "/")
	values := url.Values{}
	values.Set("timeMin", start.Format(time.RFC3339))
	values.Set("timeMax", end.Format(time.RFC3339))
	values.Set("singleEvents", "true")
	values.Set("orderBy", "startTime")
	values.Set("maxResults", fmt.Sprintf("%d", defaultMaxEvents))

	var payload struct {
		Items []struct {
			Summary string `json:"summary"`
			Start   struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"start"`
		} `json:"items"`
	}
	requestURL := baseURL + "/calendars/" + url.PathEscape(calendarID) + "/events?" + values.Encode()
	if err := c.getJSON(ctx, token, requestURL, &payload); err != nil {
		return nil, fmt.Errorf("load Google Calendar events: %w", err)
	}

	events := make([]calendarEventResult, 0, len(payload.Items))
	for _, item := range payload.Items {
		title := strings.TrimSpace(item.Summary)
		if title == "" {
			title = "Без названия"
		}
		timeLabel := "Весь день"
		sortAt := start
		if strings.TrimSpace(item.Start.DateTime) != "" {
			startTime, err := time.Parse(time.RFC3339, item.Start.DateTime)
			if err == nil {
				sortAt = startTime.In(location)
				timeLabel = sortAt.Format("15:04")
			}
		} else if strings.TrimSpace(item.Start.Date) != "" {
			startDate, err := time.ParseInLocation("2006-01-02", item.Start.Date, location)
			if err == nil {
				sortAt = startDate
			}
		}
		events = append(events, calendarEventResult{
			event:  CalendarEvent{TimeLabel: timeLabel, Title: title},
			sortAt: sortAt,
		})
	}
	return events, nil
}

func (entry calendarListEntry) canReadEvents() bool {
	if strings.TrimSpace(entry.ID) == "" || entry.Deleted || entry.Hidden {
		return false
	}
	if !entry.Primary && !entry.Selected {
		return false
	}
	switch entry.AccessRole {
	case "reader", "writer", "owner":
		return true
	default:
		return false
	}
}

func (c *Client) getJSON(ctx context.Context, token Token, requestURL string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	tokenType := strings.TrimSpace(token.TokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	request.Header.Set("Authorization", tokenType+" "+token.AccessToken)
	request.Header.Set("Accept", "application/json")

	response, err := c.client().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("Google API returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return err
	}
	return nil
}

func (c *Client) loadCredentials() (oauthClient, error) {
	if c.config.CredentialsPath == "" {
		return oauthClient{}, ErrNotConfigured
	}
	body, err := os.ReadFile(c.config.CredentialsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return oauthClient{}, ErrNotConfigured
		}
		return oauthClient{}, err
	}
	var file credentialsFile
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&file); err != nil {
		return oauthClient{}, fmt.Errorf("decode Google credentials: %w", err)
	}
	var credentials *oauthClient
	if file.Installed != nil {
		credentials = file.Installed
	} else {
		credentials = file.Web
	}
	if credentials == nil || strings.TrimSpace(credentials.ClientID) == "" {
		return oauthClient{}, fmt.Errorf("Google credentials have no OAuth client")
	}
	if credentials.AuthURI == "" {
		credentials.AuthURI = defaultAuthURI
	}
	if credentials.TokenURI == "" {
		credentials.TokenURI = defaultTokenURI
	}
	return *credentials, nil
}

func (c *Client) loadToken() (Token, error) {
	if c.config.TokenPath == "" {
		return Token{}, ErrNotAuthorized
	}
	body, err := os.ReadFile(c.config.TokenPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Token{}, ErrNotAuthorized
		}
		return Token{}, err
	}
	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return Token{}, fmt.Errorf("decode Google token: %w", err)
	}
	if token.AccessToken == "" && token.RefreshToken == "" {
		return Token{}, ErrNotAuthorized
	}
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}
	return token, nil
}

func (c *Client) saveToken(token Token) error {
	if c.config.TokenPath == "" {
		return ErrNotAuthorized
	}
	if err := os.MkdirAll(filepath.Dir(c.config.TokenPath), 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.config.TokenPath, body, 0o600)
}

func (c *Client) client() *http.Client {
	if c.config.Client != nil {
		return c.config.Client
	}
	return &http.Client{Timeout: googleRequestTimeout}
}

func (c *Client) now() time.Time {
	if c.config.Clock != nil {
		return c.config.Clock()
	}
	return time.Now()
}

func headerValue(headers []struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}, name string) string {
	for _, header := range headers {
		if strings.EqualFold(header.Name, name) {
			return header.Value
		}
	}
	return ""
}

func cleanSender(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.Index(value, "<"); index > 0 {
		value = strings.TrimSpace(value[:index])
	}
	value = strings.Trim(value, `"'`)
	return strings.TrimSpace(value)
}

func minskLocation() *time.Location {
	location, err := time.LoadLocation("Europe/Minsk")
	if err != nil {
		return time.FixedZone("Europe/Minsk", 3*60*60)
	}
	return location
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
