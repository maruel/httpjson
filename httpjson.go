// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package httpjson

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/DataDog/zstd"
	"github.com/andybalholm/brotli"
)

// HTTP client to use. For mock testing.
type Client struct {
	// Client defaults to http.DefaultClient.
	Client *http.Client
	// Logger defaults to slog.Default().
	Logger *slog.Logger
	// DefaultHeader is the headers to add to all request. For example Authorization: Bearer.
	DefaultHeader http.Header
	// Compress should be empty or one of gzip, br or zstd.
	Compress string

	_ struct{}
}

// Default will use http.DefaultClient and slog.Default().
var Default = Client{Compress: "gzip"}

// Post simplifies doing an HTTP POST in JSON.
func (c *Client) Post(ctx context.Context, url string, hdr http.Header, in, out any) error {
	start := time.Now()
	resp, err := c.PostRequest(ctx, url, hdr, in)
	if err != nil {
		return err
	}
	return c.decode(ctx, url, start, resp, out)
}

// PostRequest simplifies doing an HTTP POST in JSON. It initiates
// the requests and returns the response back.
func (c *Client) PostRequest(ctx context.Context, url string, hdr http.Header, in any) (*http.Response, error) {
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
	return c.do(req, hdr)
}

// Get does a HTTP GET and parses the returned JSON.
func (c *Client) Get(ctx context.Context, url string, hdr http.Header, out any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	start := time.Now()
	resp, err := c.do(req, hdr)
	if err != nil {
		return err
	}
	return c.decode(ctx, url, start, resp, out)
}

//

func (c *Client) do(req *http.Request, hdr http.Header) (*http.Response, error) {
	// The standard library includes gzip. Disable transparent compression and
	// add br and zstd.
	req.Header.Set("Accept-Encoding", "gzip, br, zstd")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	for k, v := range c.DefaultHeader {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}
	for k, v := range hdr {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

func (c *Client) decode(ctx context.Context, url string, start time.Time, resp *http.Response, out any) (err error) {
	defer func() {
		if err2 := resp.Body.Close(); err == nil {
			err = err2
		}
	}()
	var body io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gz, err2 := gzip.NewReader(resp.Body)
		if err2 != nil {
			return
		}
		defer func() {
			if err3 := gz.Close(); err == nil {
				err = err3
			}
		}()
		body = gz
	case "br":
		body = brotli.NewReader(resp.Body)
	case "zstd":
		zs := zstd.NewReader(resp.Body)
		defer func() {
			if err3 := zs.Close(); err == nil {
				err = err3
			}
		}()
		body = zs
	default:
		body = resp.Body
	}
	b, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("failed to read server response: %w", err)
	}

	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	// Try to decode before checking the status code.
	if err = d.Decode(out); err != nil {
		err = fmt.Errorf("failed to decode server response: %w", err)
		c.getLogger().ErrorContext(ctx, "httpjson", "url", url, "duration", time.Since(start), "code", resp.StatusCode, "err", err)
		// This is a REST API call failure. The data probably has an "error"
		// field that needs to be decoded manually. Pass it to the caller.
		return &Error{URL: url, Err: err, ResponseBody: b, StatusCode: resp.StatusCode, Status: resp.Status}
	}
	if resp.StatusCode >= 400 {
		c.getLogger().ErrorContext(ctx, "httpjson", "url", url, "duration", time.Since(start), "code", resp.StatusCode, "err", err)
		return &Error{URL: url, ResponseBody: b, StatusCode: resp.StatusCode, Status: resp.Status}
	}
	c.getLogger().DebugContext(ctx, "httpjson", "url", url, "duration", time.Since(start))
	return nil
}

func (c *Client) getLogger() *slog.Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return slog.Default()
}

// Error represents an HTTP request that returned an HTTP error.
// It contains the response body if any.
type Error struct {
	URL          string
	Err          error
	ResponseBody []byte
	StatusCode   int
	Status       string
}

func (h *Error) Error() string {
	if h.Err != nil {
		return h.Err.Error()
	}
	return h.Status
}

func (h *Error) Unwrap() error {
	return h.Err
}
