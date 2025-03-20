// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers_test

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/maruel/httpjson"
	"github.com/maruel/httpjson/roundtrippers"
)

func ExampleLog() {
	// Example on how to hook into the HTTP client roundtripper to log each HTTP
	// request.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"message": "Working"}`))
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout,
		&slog.HandlerOptions{
			Level: slog.LevelDebug,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				// For testing reproducibility, remove the timestamp, url, request id and duration.
				if a.Key == "time" || a.Key == "url" || a.Key == "id" || a.Key == "dur" {
					return slog.Attr{}
				}
				return a
			},
		}))

	t := &roundtrippers.Log{Transport: http.DefaultTransport, L: logger}
	c := httpjson.Client{Client: &http.Client{Transport: t}}

	var out struct {
		Message string `json:"message"`
	}
	if err := c.Get(context.Background(), ts.URL, nil, &out); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %q\n", out.Message)
	// Output:
	// level=INFO msg=http method=GET Content-Encoding=""
	// level=INFO msg=http status=200 Content-Encoding="" Content-Length=22 Content-Type="application/json; charset=utf-8"
	// level=INFO msg=http size=22 err=<nil>
	// Response: "Working"
}

func ExampleCapture() {
	// Example on how to hook into the HTTP client roundtripper to capture each HTTP
	// request.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"message": "Working"}`))
	}))
	defer ts.Close()

	ch := make(chan roundtrippers.Record, 1)
	t := &roundtrippers.Capture{Transport: http.DefaultTransport, C: ch}
	c := httpjson.Client{Client: &http.Client{Transport: t}}

	var out struct {
		Message string `json:"message"`
	}
	if err := c.Get(context.Background(), ts.URL, nil, &out); err != nil {
		log.Fatal(err)
	}

	// Print the captured request and response.
	record := <-ch
	fmt.Printf("Response body: %q\n", record.Response.Body)

	fmt.Printf("Response: %q\n", out.Message)
	// Output:
	// Response body: {"{\"message\": \"Working\"}"}
	// Response: "Working"
}
