// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package httpjson_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/maruel/httpjson"
)

func acceptCompressed(r *http.Request, want string) bool {
	for encoding := range strings.SplitSeq(r.Header.Get("Accept-Encoding"), ",") {
		if strings.TrimSpace(encoding) == want {
			return true
		}
	}
	return false
}

// serverRespondQuestion is an example of a server API implementation that
// returns different structure based on the input. That happens frequently in
// servers implemented with dynamic languages (python, ruby, nodejs, etc).
func serverRespondQuestion(question string) any {
	switch question {
	case "weather":
		return map[string]string{"message": "Comfortable"}
	default:
		return map[string]string{"error": "I only answer weather questions", "got": question}
	}
}

func handleGetCompressed(w http.ResponseWriter, r *http.Request) {
	if !acceptCompressed(r, "zstd") {
		http.Error(w, "sorry, I only talk zstd", http.StatusBadRequest)
		return
	}
	out := serverRespondQuestion(r.URL.Query().Get("question"))
	data, err := json.Marshal(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Encoding", "zstd")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	c, err := zstd.NewWriter(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err = c.Write(data); err != nil {
		slog.WarnContext(r.Context(), "http", "req", "r", "err", err)
	}
	if err = c.Close(); err != nil {
		slog.WarnContext(r.Context(), "http", "req", "r", "err", err)
	}
}

func ExampleClient_Get() {
	// Compression is transparently supported!
	ts := httptest.NewServer(http.HandlerFunc(handleGetCompressed))
	defer ts.Close()

	var out struct {
		Message string `json:"message"`
	}
	if err := httpjson.DefaultClient.Get(context.Background(), ts.URL+"?question=weather", nil, &out); err != nil {
		// Handle various kinds of errors.
		var herr *httpjson.Error
		if errors.As(err, &herr) {
			fmt.Printf("httpjson.Error: body=%q code=%d", herr.ResponseBody, herr.StatusCode)
		}
		var jerr *json.SyntaxError
		if errors.As(err, &jerr) {
			fmt.Printf("json.SyntaxError: offset=%d", jerr.Offset)
		}
		// Print the error as a generic error.
		fmt.Printf("Error: %s\n", err)
		return
	}

	fmt.Printf("Response: %s\n", out.Message)
	// Output: Response: Comfortable
}

func ExampleClient_GetRequest() {
	ts := httptest.NewServer(http.HandlerFunc(handleGetCompressed))
	defer ts.Close()

	var out struct {
		Message string `json:"message"`
	}
	var fallback struct {
		Error string `json:"error"`
		Got   string `json:"got"`
	}
	resp, err := httpjson.DefaultClient.GetRequest(context.Background(), ts.URL+"?question=life", nil)
	if err != nil {
		log.Fatal(err)
	}
	i, err := httpjson.DecodeResponse(resp, &out, &fallback)
	switch i {
	case 0:
		fmt.Printf("Success case: %s\n", out.Message)
	case 1:
		fmt.Printf("Fallback: %s\n", fallback.Error)
	case -1:
		// No decoding happened. Handle various kinds of errors.
		var herr *httpjson.Error
		if errors.As(err, &herr) {
			fmt.Printf("httpjson.Error: body=%q code=%d", herr.ResponseBody, herr.StatusCode)
		}
		var jerr *json.SyntaxError
		if errors.As(err, &jerr) {
			fmt.Printf("json.SyntaxError: offset=%d", jerr.Offset)
		}
		fmt.Printf("Error: %s\n", err)
	}
	// Output: Fallback: I only answer weather questions
}

func handlePostCompressed(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Encoding") != "gzip" {
		http.Error(w, "expected gzip", http.StatusBadRequest)
		return
	}
	if !acceptCompressed(r, "zstd") {
		http.Error(w, "sorry, I only talk zstd", http.StatusBadRequest)
		return
	}
	gz, err := gzip.NewReader(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var in struct {
		Question string `json:"question"`
	}
	var out any
	if err = json.NewDecoder(gz).Decode(&in); err != nil {
		out = map[string]string{"error": err.Error()}
	} else {
		out = serverRespondQuestion(in.Question)
	}
	data, err := json.Marshal(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Encoding", "zstd")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	c, err := zstd.NewWriter(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err = c.Write(data); err != nil {
		slog.WarnContext(r.Context(), "http", "req", "r", "err", err)
	}
	if err = c.Close(); err != nil {
		slog.WarnContext(r.Context(), "http", "req", "r", "err", err)
	}
}

func ExampleClient_Post() {
	ts := httptest.NewServer(http.HandlerFunc(handlePostCompressed))
	defer ts.Close()

	ctx := context.Background()
	h := http.Header{}
	h.Set("Authentication", "Bearer 123")
	in := map[string]string{"question": "weather"}
	out := map[string]string{}
	// Transparently compress HTTP POST content.
	c := httpjson.DefaultClient
	c.PostCompress = "gzip"
	if err := c.Post(ctx, ts.URL, h, in, &out); err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	fmt.Printf("Response: %s\n", out["message"])
	// Output: Response: Comfortable
}

func ExampleClient_PostRequest() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if req := r.Header.Get("Authentication"); req != "Bearer AAA" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": "Unauthorized"}`))
		} else {
			_, _ = w.Write([]byte(`{"message": "Comfortable"}`))
		}
	}))
	defer ts.Close()

	ctx := context.Background()
	h := http.Header{}
	h.Set("Authentication", "Bearer 123")
	in := map[string]string{"question": "weather"}
	resp, err := httpjson.DefaultClient.PostRequest(ctx, ts.URL, h, in)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	// Useful to stream the response. This is also useful if you want to allow
	// unknown fields.
	var out struct {
		Message string `json:"message"`
	}
	var fallback struct {
		Error string `json:"error"`
	}
	i, err := httpjson.DecodeResponse(resp, &out, &fallback)
	switch i {
	case 0:
		fmt.Printf("Success case: %s\n", out.Message)
	case 1:
		fmt.Printf("Structured Error: %s\n", fallback.Error)
	case -1:
		// No decoding happened. Handle various kinds of errors.
		var herr *httpjson.Error
		if errors.As(err, &herr) {
			fmt.Printf("httpjson.Error: body=%q code=%d", herr.ResponseBody, herr.StatusCode)
		}
		var jerr *json.SyntaxError
		if errors.As(err, &jerr) {
			fmt.Printf("json.SyntaxError: offset=%d", jerr.Offset)
		}
		fmt.Printf("Error: %s\n", err)
	}
	// Output: Structured Error: Unauthorized
}

type LoggingRoundTripper struct {
	R http.RoundTripper
	L *slog.Logger
}

func genID() string {
	var bytes [12]byte
	rand.Read(bytes[:])
	return base64.RawURLEncoding.EncodeToString(bytes[:])
}

func (l *LoggingRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	fmt.Printf("OnRequest: %s\n", r.Method)
	ctx := r.Context()
	start := time.Now()
	ll := l.L.With("id", genID())
	ll.DebugContext(ctx, "http", "url", r.URL.String(), "method", r.Method, "Content-Encoding", r.Header.Get("Content-Encoding"))
	resp, err := l.R.RoundTrip(r)
	if err != nil {
		fmt.Printf("OnResponse: %q\n", err)
		ll.ErrorContext(ctx, "http", "duration", time.Since(start), "err", err)
	} else {
		ce := resp.Header.Get("Content-Encoding")
		cl := resp.Header.Get("Content-Length")
		ct := resp.Header.Get("Content-Type")
		fmt.Printf("OnResponse: %q; Content-Encoding: %q Content-Length: %q Content-Type: %q\n", resp.Status, ce, cl, ct)
		ll.InfoContext(ctx, "http", "duration", time.Since(start), "status", resp.StatusCode, "Content-Encoding", ce, "Content-Length", cl, "Content-Type", ct)
		resp.Body = &loggingBody{r: resp.Body, ctx: ctx, start: start, l: ll}
	}
	return resp, err
}

type loggingBody struct {
	r     io.ReadCloser
	ctx   context.Context
	start time.Time
	l     *slog.Logger

	responseSize    int64
	responseContent bytes.Buffer
	err             error
}

func (l *loggingBody) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	if n > 0 {
		l.responseSize += int64(n)
		_, _ = l.responseContent.Write(p[:n])
	}
	if err != nil && err != io.EOF && l.err == nil {
		l.err = err
	}
	return n, err
}

func (l *loggingBody) Close() error {
	err := l.r.Close()
	if err != nil && l.err == nil {
		l.err = err
	}
	fmt.Printf("Body: %q  err=%v\n", l.responseContent.String(), err)
	if l.err != nil {
		l.l.ErrorContext(l.ctx, "http", "duration", time.Since(l.start), "size", l.responseSize, "err", l.err)
	} else {
		l.l.InfoContext(l.ctx, "http", "duration", time.Since(l.start), "size", l.responseSize)
	}
	return err
}

func ExampleClient_logging() {
	// Example on how to hook into the HTTP client roundtripper to do logging of
	// each HTTP request.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"message": "Working"}`))
	}))
	defer ts.Close()

	// Unsolicited recommendation for CLI tools: github.com/lmittmann/tint along
	// with github.com/mattn/go-colorable is great for console output.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// LoggingRoundTripper prints deterministic information to stdout for the
	// test to capture. Remove the fmt.Printf() calls in production.
	//
	// It captures all of the response body in memory, which is not recommended
	// for large responses. Remove loggingBody.responseContent if you don't need the body.
	t := &LoggingRoundTripper{R: http.DefaultTransport, L: logger}
	c := httpjson.Client{Client: &http.Client{Transport: t}}

	var out struct {
		Message string `json:"message"`
	}
	if err := c.Get(context.Background(), ts.URL, nil, &out); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %q\n", out.Message)
	// Output:
	// OnRequest: GET
	// OnResponse: "200 OK"; Content-Encoding: "" Content-Length: "22" Content-Type: "application/json; charset=utf-8"
	// Body: "{\"message\": \"Working\"}"  err=<nil>
	// Response: "Working"
}
