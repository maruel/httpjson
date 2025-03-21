// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package httpjson is a deceptively simple JSON REST HTTP client.
package httpjson

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// Client is a JSON REST HTTP client supporting compression and using good
// default behavior.
type Client struct {
	// Client defaults to http.DefaultClient. Overridde to add logging on each
	// HTTP request. See the Logging example.
	Client *http.Client
	// DefaultHeader is the headers to add to all request. For example "Authorization: Bearer 123".
	DefaultHeader http.Header
	// PostCompress determines HTTP POST compression. It must be empty or one of: "gzip", "br" or "zstd".
	//
	// Warning ⚠: compressing POST content is not supported on most servers.
	PostCompress string
	// Lenient allows unknown fields in the response.
	//
	// This inhibits from calling DisallowUnknownFields() on the JSON decoder.
	//
	// Use this in production so that your client doesn't break when the server
	// add new fields.
	Lenient bool

	_ struct{}
}

// DefaultClient uses http.DefaultClient and does no compression.
var DefaultClient = Client{}

// Get simplifies doing an HTTP GET in JSON.
//
// It transparently support advanced compression.
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
// The result is transparently decompressed.
func (c *Client) GetRequest(ctx context.Context, url string, hdr http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req, hdr)
}

// Post simplifies doing an HTTP POST in JSON.
//
// It transparently support advanced compression.
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
// The result is transparently decompressed.
// Buffers post data in memory.
func (c *Client) PostRequest(ctx context.Context, url string, hdr http.Header, in any) (*http.Response, error) {
	b := bytes.Buffer{}
	var w io.Writer = &b
	var cl io.Closer
	switch c.PostCompress {
	case "gzip":
		gz := gzip.NewWriter(&b)
		w = gz
		cl = gz
	case "br":
		br := brotli.NewWriter(&b)
		w = br
		cl = br
	case "zstd":
		zs, err := zstd.NewWriter(&b)
		if err != nil {
			return nil, err
		}
		w = zs
		cl = zs
	case "":
	default:
		return nil, fmt.Errorf("invalid PostCompress value: %q", c.PostCompress)
	}
	e := json.NewEncoder(w)
	// OMG this took me a while to figure this out. This affects LLM token encoding.
	e.SetEscapeHTML(false)
	if err := e.Encode(in); err != nil {
		return nil, fmt.Errorf("internal error: %w", err)
	}
	if cl != nil {
		if err := cl.Close(); err != nil {
			return nil, fmt.Errorf("internal error: %w", err)
		}
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, &b)
	if err != nil {
		return nil, err
	}
	if c.PostCompress != "" {
		if hdr == nil {
			hdr = make(http.Header)
		} else {
			hdr = hdr.Clone()
		}
		hdr.Set("Content-Encoding", c.PostCompress)
	}
	return c.Do(req, hdr)
}

// Do sets the correct headers and transparently decompresses the response.
func (c *Client) Do(req *http.Request, hdr http.Header) (*http.Response, error) {
	// The standard library includes gzip. Disable transparent compression and
	// add br and zstd. Tell the server we prefer zstd.
	req.Header.Set("Accept-Encoding", "zstd, br, gzip")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	for k, v := range c.DefaultHeader {
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
	resp, err := client.Do(req)
	if resp != nil {
		ce := resp.Header.Get("Content-Encoding")
		switch ce {
		case "br":
			resp.Body = &body{r: brotli.NewReader(resp.Body), c: []io.Closer{resp.Body}}
		case "gzip":
			gz, err2 := gzip.NewReader(resp.Body)
			if err2 != nil {
				return resp, errors.Join(err2, err)
			}
			resp.Body = &body{r: gz, c: []io.Closer{resp.Body, gz}}
		case "zstd":
			zs, err2 := zstd.NewReader(resp.Body)
			if err2 != nil {
				return resp, errors.Join(err2, err)
			}
			resp.Body = &body{r: zs, c: []io.Closer{resp.Body, &adapter{zs}}}
		}
	}
	return resp, err
}

type adapter struct {
	zs *zstd.Decoder
}

func (a *adapter) Close() error {
	// zstd.Decoder doesn't implement io.Closer. :/
	a.zs.Close()
	return nil
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

type body struct {
	r io.Reader
	c []io.Closer
}

func (b *body) Read(p []byte) (n int, err error) {
	return b.r.Read(p)
}

func (b *body) Close() error {
	var errs []error
	// Close in reverse order.
	for i := len(b.c) - 1; i >= 0; i-- {
		if err := b.c[i].Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
