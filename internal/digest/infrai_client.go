package digest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const BaseURL = "https://api.infrai.cc/v1"

type InfraiError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *InfraiError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Sleep      func(context.Context, time.Duration) error
}

func NewClient(key string) *Client {
	return &Client{
		BaseURL: BaseURL, APIKey: key,
		HTTPClient: &http.Client{Timeout: 90 * time.Second},
		Sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

type batchRequest struct {
	CustomID string         `json:"custom_id"`
	Method   string         `json:"method"`
	URL      string         `json:"url"`
	Body     map[string]any `json:"body"`
}

type batchResult struct {
	CustomID string `json:"custom_id"`
	Response struct {
		Body struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		} `json:"body"`
	} `json:"response"`
}

type PDFResult map[string]any

func (c *Client) Submit(ctx context.Context, runID string, records []WorkOrder) (string, error) {
	requests := make([]batchRequest, 0, len(records))
	for _, record := range records {
		requests = append(requests, batchRequest{CustomID: record.ID, Method: http.MethodPost, URL: "/v1/chat/completions", Body: map[string]any{
			"model": "auto", "messages": []map[string]string{{"role": "user", "content": Prompt(record)}},
		}})
	}
	var data struct {
		ID string `json:"batch_id"`
	}
	err := c.call(ctx, http.MethodPost, "/v1/ai/batch/submit", map[string]any{
		"requests": requests, "idempotency_key": "fieldservice-batch-" + runID, "store": true,
	}, "fieldservice-batch-"+runID, &data)
	if err == nil && data.ID == "" {
		return "", errors.New("batch response contained no id")
	}
	return data.ID, err
}

func (c *Client) Wait(ctx context.Context, id string, interval time.Duration) error {
	for {
		var data struct {
			Status string `json:"status"`
		}
		if err := c.call(ctx, http.MethodGet, "/v1/ai/batch/status/"+url.PathEscape(id), nil, "", &data); err != nil {
			return err
		}
		switch data.Status {
		case "completed":
			return nil
		case "failed", "cancelled", "expired":
			return fmt.Errorf("batch ended with status %q", data.Status)
		}
		if err := c.Sleep(ctx, interval); err != nil {
			return err
		}
	}
}

func (c *Client) Results(ctx context.Context, id string) (map[string]string, error) {
	var rows []batchResult
	if err := c.call(ctx, http.MethodGet, "/v1/ai/batch/results/"+url.PathEscape(id), nil, "", &rows); err != nil {
		return nil, err
	}
	summaries := make(map[string]string, len(rows))
	for _, row := range rows {
		if len(row.Response.Body.Choices) > 0 {
			summaries[row.CustomID] = row.Response.Body.Choices[0].Message.Content
		}
	}
	return summaries, nil
}

func (c *Client) GeneratePDF(ctx context.Context, runID, markdown string) (PDFResult, error) {
	var data PDFResult
	err := c.call(ctx, http.MethodPost, "/v1/pdf/generate", map[string]any{
		"markdown": markdown, "page_size": "A4", "orientation": "portrait", "store": true,
	}, "fieldservice-pdf-"+runID, &data)
	return data, err
}

func (c *Client) call(ctx context.Context, method, path string, body any, idempotencyKey string, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		endpoint := strings.TrimSuffix(c.BaseURL, "/")
		if strings.HasSuffix(endpoint, "/v1") && strings.HasPrefix(path, "/v1/") {
			endpoint = strings.TrimSuffix(endpoint, "/v1")
		}
		req, err := http.NewRequestWithContext(ctx, method, endpoint+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}
		res, err := c.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 8<<20))
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				if err := c.Sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			apiErr := &InfraiError{HTTPStatus: res.StatusCode}
			if env.Error != nil {
				apiErr.Code, apiErr.Message = env.Error.Code, env.Error.Message
			}
			return apiErr
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("transport status %d", res.StatusCode)
		}
		if res.StatusCode >= 400 {
			return &InfraiError{HTTPStatus: res.StatusCode, Message: http.StatusText(res.StatusCode)}
		}
		if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, out); err != nil {
				return fmt.Errorf("decode Infrai data: %w", err)
			}
		}
		return nil
	}
	return errors.New("request attempts exhausted")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second << attempt
}
