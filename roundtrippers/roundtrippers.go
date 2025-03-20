// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Log is a http.RoundTripper that logs each request and response via slog.
// It defaults to slog.LevelInfo level unless an error is returned from the
// roundtripper, then the final log is logged at error level.
type Log struct {
	Transport http.RoundTripper
	L         *slog.Logger
	Level     slog.Level

	_ struct{}
}

func (l *Log) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	ll := l.L.With("id", genID(), "dur", elapsedTimeValue{start: time.Now()})
	ll.Log(ctx, l.Level, "http", "url", req.URL.String(), "method", req.Method, "Content-Encoding", req.Header.Get("Content-Encoding"))
	resp, err := l.Transport.RoundTrip(req)
	if err != nil {
		ll.ErrorContext(ctx, "http", "err", err)
	} else {
		ce := resp.Header.Get("Content-Encoding")
		cl := resp.Header.Get("Content-Length")
		ct := resp.Header.Get("Content-Type")
		ll.Log(ctx, l.Level, "http", "status", resp.StatusCode, "Content-Encoding", ce, "Content-Length", cl, "Content-Type", ct)
		resp.Body = &logBody{body: resp.Body, ctx: ctx, l: ll, level: l.Level}
	}
	return resp, err
}

type logBody struct {
	body  io.ReadCloser
	ctx   context.Context
	l     *slog.Logger
	level slog.Level

	responseSize int64
	err          error
}

func (l *logBody) Read(p []byte) (int, error) {
	n, err := l.body.Read(p)
	if n > 0 {
		l.responseSize += int64(n)
	}
	if err != nil && err != io.EOF && l.err == nil {
		l.err = err
	}
	return n, err
}

func (l *logBody) Close() error {
	err := l.body.Close()
	if err != nil && l.err == nil {
		l.err = err
	}
	level := l.level
	if l.err != nil {
		level = slog.LevelError
	}
	l.l.Log(l.ctx, level, "http", "size", l.responseSize, "err", l.err)
	return err
}

//

// Record is a captured HTTP request and response by the Capture http.RoundTripper.
type Record struct {
	Request  *http.Request
	Response *http.Response
	Err      error

	_ struct{}
}

// Capture is a http.RoundTripper that records each request.
type Capture struct {
	Transport http.RoundTripper
	C         chan<- Record

	_ struct{}
}

func (c *Capture) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := c.Transport.RoundTrip(req)
	if resp != nil {
		// Make a copy of the response.
		resp2 := &http.Response{}
		*resp2 = *resp
		resp.Body = &captureBody{
			body:    resp.Body,
			resp:    resp2,
			c:       c.C,
			content: &bytes.Buffer{},
		}
	} else {
		c.C <- Record{Request: req, Err: err}
	}
	return resp, err
}

type captureBody struct {
	body    io.ReadCloser
	resp    *http.Response
	c       chan<- Record
	content *bytes.Buffer
	err     error
}

func (c *captureBody) Read(p []byte) (int, error) {
	n, err := c.body.Read(p)
	_, _ = c.content.Write(p[:n])
	if err != nil && err != io.EOF && c.err == nil {
		c.err = err
	}
	return n, err
}

func (l *captureBody) Close() error {
	err := l.body.Close()
	l.resp.Body = io.NopCloser(l.content)
	l.c <- Record{Request: l.resp.Request, Response: l.resp, Err: l.err}
	return err
}

//

func genID() string {
	var bytes [12]byte
	rand.Read(bytes[:])
	return base64.RawURLEncoding.EncodeToString(bytes[:])
}

type elapsedTimeValue struct {
	start time.Time
}

func (v elapsedTimeValue) LogValue() slog.Value {
	return slog.DurationValue(time.Since(v.start))
}
