// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package httpjson is a deceptively simple JSON REST HTTP client.
package httpjson

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Client is a JSON REST HTTP client using good default behavior.
type Client struct {
	// Client defaults to http.DefaultClient. Override with http.RoundTripper to
	// add functionality. See github.com/maruel/roundtrippers for useful ones
	// like logging, compression and more!
	Client *http.Client
	// Lenient allows unknown fields in the response.
	//
	// This inhibits from calling DisallowUnknownFields() on the JSON decoder.
	//
	// Use this in production so that your client doesn't break when the server
	// add new fields.
	Lenient bool

	_ struct{}
}

// DefaultClient uses http.DefaultClient and refuses unknown fields.
var DefaultClient = Client{}

// Get simplifies doing an HTTP GET in JSON.
//
// It fails on unknown fields in the response.
// Buffers response body in memory.
func (c *Client) Get(ctx context.Context, url string, hdr http.Header, out any) error {
	resp, err := c.GetRequest(ctx, url, hdr)
	if err != nil {
		return err
	}
	return c.decodeResponse(resp, out)
}

// GetRequest simplifies doing an HTTP POST in JSON.
//
// It initiates the requests and returns the response back for further processing.
func (c *Client) GetRequest(ctx context.Context, url string, hdr http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req, hdr)
}

// Post simplifies doing an HTTP POST in JSON.
//
// It fails on unknown fields in the response.
// Buffers both post data and response body in memory.
func (c *Client) Post(ctx context.Context, url string, hdr http.Header, in, out any) error {
	resp, err := c.PostRequest(ctx, url, hdr, in)
	if err != nil {
		return err
	}
	return c.decodeResponse(resp, out)
}

// PostRequest simplifies doing an HTTP POST in JSON.
//
// It initiates the requests and returns the response back for further processing.
// Buffers post data in memory.
func (c *Client) PostRequest(ctx context.Context, url string, hdr http.Header, in any) (*http.Response, error) {
	b := bytes.Buffer{}
	e := json.NewEncoder(&b)
	// OMG this took me a while to figure this out. This affects LLM token encoding.
	e.SetEscapeHTML(false)
	if err := e.Encode(in); err != nil {
		return nil, fmt.Errorf("internal error: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, &b)
	if err != nil {
		return nil, err
	}
	return c.Do(req, hdr)
}

// Do sets the correct headers and allow adding per-request headers.
func (c *Client) Do(req *http.Request, hdr http.Header) (*http.Response, error) {
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	for k, v := range hdr {
		switch len(v) {
		case 0:
			req.Header.Del(k)
		case 1:
			req.Header.Set(k, v[0])
		default:
			for _, vv := range v {
				req.Header.Add(k, vv)
			}
		}
	}
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

// DecodeResponse parses the response body as JSON, trying strict decoding for
// each of the output struct passed in, falling back as the decoding fails. It
// then closes the response body.
//
// Returns the index of which output structured was decoded along joined errors
// for both json decode failure (*json.UnmarshalTypeError, *json.SyntaxError,
// *json.InvalidUnmarshalError) and HTTP status code (*httpjson.Error). Returns
// -1 as the index if no output was decoded.
//
// Buffers response body in memory.
func DecodeResponse(resp *http.Response, out ...any) (int, error) {
	res := -1
	b, err := io.ReadAll(resp.Body)
	if err2 := resp.Body.Close(); err == nil {
		err = err2
	}
	if err != nil {
		return res, fmt.Errorf("failed to read server response: %w", err)
	}
	var errs []error
	for i := range out {
		d := json.NewDecoder(bytes.NewReader(b))
		d.DisallowUnknownFields()
		d.UseNumber()
		if err = d.Decode(out[i]); err == nil {
			res = i
			break
		}
		errs = append(errs, fmt.Errorf("failed to decode server response option #%d as type %T: %w", i, out[i], err))
	}
	if len(errs) != 0 || resp.StatusCode >= 400 {
		// Include the body in case of error so the user can diagnose.
		errs = append(errs, &Error{ResponseBody: b, StatusCode: resp.StatusCode, Status: resp.Status, PrintBody: len(errs) != 0})
	}
	return res, errors.Join(errs...)
}

func (c *Client) decodeResponse(resp *http.Response, out any) error {
	b, err := io.ReadAll(resp.Body)
	if err2 := resp.Body.Close(); err == nil {
		err = err2
	}
	if err != nil {
		return fmt.Errorf("failed to read server response: %w", err)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if !c.Lenient {
		d.DisallowUnknownFields()
	}
	d.UseNumber()
	return d.Decode(out)
}

// Error represents an HTTP request that returned an HTTP error.
// It contains the response body if any.
type Error struct {
	ResponseBody []byte
	StatusCode   int
	Status       string
	PrintBody    bool
}

// Error implements error, returning "http <status code>".
func (h *Error) Error() string {
	out := fmt.Sprintf("http %d", h.StatusCode)
	if h.PrintBody {
		out += "\n" + string(h.ResponseBody)
	}
	return out
}
