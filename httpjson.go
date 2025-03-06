// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package httpjson

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// HTTP client to use. For mock testing.
type Client struct {
	Client *http.Client
	Logger *slog.Logger
}

// Default will use http.DefaultClient and slog.Default().
var Default = Client{}

// Post simplifies doing an HTTP POST in JSON.
func (c *Client) Post(ctx context.Context, url string, in, out any) error {
	start := time.Now()
	resp, err := c.PostRequest(ctx, url, in)
	if err != nil {
		return err
	}
	return c.decode(ctx, url, start, resp, out)
}

// PostRequest simplifies doing an HTTP POST in JSON. It initiates
// the requests and returns the response back.
func (c *Client) PostRequest(ctx context.Context, url string, in any) (*http.Response, error) {
	b := bytes.Buffer{}
	e := json.NewEncoder(&b)
	// OMG this took me a while to figure this out. This affects token encoding.
	e.SetEscapeHTML(false)
	if err := e.Encode(in); err != nil {
		return nil, fmt.Errorf("internal error: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, &b)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

// Get does a HTTP GET and parses the returned JSON.
func (c *Client) Get(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	start := time.Now()
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	return c.decode(ctx, url, start, resp, out)
}

//

func (c *Client) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Content-Type", "application/json")
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

func (c *Client) decode(ctx context.Context, url string, start time.Time, resp *http.Response, out any) error {
	defer resp.Body.Close()
	var errs []error
	if b, err := io.ReadAll(resp.Body); err != nil {
		errs = append(errs, fmt.Errorf("failed to read server response: %w", err))
	} else {
		d := json.NewDecoder(bytes.NewReader(b))
		d.DisallowUnknownFields()
		if err = d.Decode(out); err != nil {
			c.getLogger().ErrorContext(ctx, "httpjson", "url", url, "duration", time.Since(start), "resp", string(b), "err", err)
			errs = append(errs, fmt.Errorf("failed to decode server response: %w", err))
		}
	}
	if resp.StatusCode >= 400 {
		c.getLogger().ErrorContext(ctx, "httpjson", "url", url, "duration", time.Since(start), "code", resp.StatusCode)
		errs = append(errs, &Error{URL: url, StatusCode: resp.StatusCode, Status: resp.Status})
	}
	if len(errs) == 0 {
		c.getLogger().DebugContext(ctx, "httpjson", "url", url, "duration", time.Since(start))
		return nil
	}
	return errors.Join(errs...)
}

func (c *Client) getLogger() *slog.Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return slog.Default()
}

// Error represents an HTTP request that returned an HTTP error.
type Error struct {
	URL        string
	StatusCode int
	Status     string
}

func (h *Error) Error() string {
	return h.Status
}
