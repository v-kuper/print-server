package telegramfax

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrBusinessConnectionNotFound = errors.New("telegram business connection not found")

type Client interface {
	GetUpdates(context.Context, GetUpdatesRequest) ([]Update, error)
	GetBusinessConnection(context.Context, string) (BusinessConnection, error)
}

type HTTPClient struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

func NewHTTPClient(token string, baseURL string, client *http.Client) *HTTPClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultAPIBaseURL
	}
	if client == nil {
		client = &http.Client{Timeout: 35 * time.Second}
	}
	return &HTTPClient{
		token:      strings.TrimSpace(token),
		baseURL:    baseURL,
		httpClient: client,
	}
}

func (c *HTTPClient) GetUpdates(ctx context.Context, request GetUpdatesRequest) ([]Update, error) {
	return doTelegram[[]Update](ctx, c, "getUpdates", request)
}

func (c *HTTPClient) GetBusinessConnection(ctx context.Context, id string) (BusinessConnection, error) {
	return doTelegram[BusinessConnection](ctx, c, "getBusinessConnection", map[string]string{
		"business_connection_id": id,
	})
}

type telegramResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	ErrorCode   int    `json:"error_code,omitempty"`
	Description string `json:"description,omitempty"`
}

func doTelegram[T any](ctx context.Context, client *HTTPClient, method string, payload any) (T, error) {
	var zero T
	body, err := json.Marshal(payload)
	if err != nil {
		return zero, err
	}
	url := client.baseURL + "/bot" + client.token + "/" + method
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return zero, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return zero, err
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return zero, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return zero, telegramAPIError(method, response.StatusCode, data)
	}
	var parsed telegramResponse[T]
	if err := json.Unmarshal(data, &parsed); err != nil {
		return zero, fmt.Errorf("decode Telegram %s response: %w", method, err)
	}
	if !parsed.OK {
		return zero, telegramAPIError(method, parsed.ErrorCode, []byte(parsed.Description))
	}
	return parsed.Result, nil
}

func telegramAPIError(method string, code int, data []byte) error {
	description := strings.TrimSpace(string(data))
	if strings.Contains(strings.ToLower(description), "business connection") &&
		strings.Contains(strings.ToLower(description), "not found") {
		return ErrBusinessConnectionNotFound
	}
	if description == "" {
		description = "empty response"
	}
	return fmt.Errorf("telegram %s failed with code %d: %s", method, code, description)
}
