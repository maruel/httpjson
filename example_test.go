// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package httpjson_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/DataDog/zstd"
	"github.com/maruel/httpjson"
)

func ExampleClient_Get() {
	// Compression is transparently supported, including zstd.
	c := httpjson.DefaultClient
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		// Simplified version, server should adapt according to the client's "Accept-Encoding".
		w.Header().Set("Content-Encoding", "zstd")
		z := zstd.NewWriter(w)
		if _, err := z.Write([]byte(`{"message": "Comfortable"}`)); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := z.Close(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}))
	defer ts.Close()

	ctx := context.Background()
	var out struct {
		Message string `json:"message"`
	}
	if err := c.Get(ctx, ts.URL, nil, &out); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %s\n", out.Message)
	// Output: Response: Comfortable
}

func ExampleClient_Get_error_with_fallback() {
	c := httpjson.DefaultClient
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"detail":"confused"}}`))
	}))
	defer ts.Close()

	ctx := context.Background()
	var out struct {
		Message string `json:"message"`
	}
	if err := c.Get(ctx, ts.URL, nil, &out); err != nil {
		if err, ok := err.(*httpjson.Error); ok {
			// Decode a fallback.
			if err.StatusCode == http.StatusBadRequest {
				var err2 struct {
					Error struct {
						Detail string `json:"detail"`
					} `json:"error"`
				}
				d := json.NewDecoder(bytes.NewReader(err.ResponseBody))
				d.DisallowUnknownFields()
				if err := d.Decode(&err2); err == nil {
					// This is the final code path taken to handle the structured server error response.
					fmt.Printf("Error: %s\n", err2.Error.Detail)
				} else {
					log.Fatal(err)
				}
			} else {
				log.Fatal(err)
			}
		} else {
			log.Fatal("unexpected")
		}
	} else {
		log.Fatal("unexpected")
	}
	// Output: Error: confused
}

func ExampleClient_Post() {
	c := httpjson.DefaultClient
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		// Simplified version, server should adapt according to the client's request.
		if r.Header.Get("Content-Encoding") != "gzip" {
			http.Error(w, "expected gzip", http.StatusBadRequest)
			return
		}
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		in := map[string]string{}
		if err := json.NewDecoder(gz).Decode(&in); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if in["question"] != "weather" {
			http.Error(w, "expected question=weather", http.StatusBadRequest)
			return
		}
		if _, err := w.Write([]byte(`{"message": "Comfortable"}`)); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}))
	defer ts.Close()

	ctx := context.Background()
	h := http.Header{}
	h.Set("Authentication", "Bearer 123")
	in := map[string]string{"question": "weather"}
	out := map[string]string{}
	if err := c.Post(ctx, ts.URL, h, in, &out); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %s\n", out["message"])
	// Output: Response: Comfortable
}

func ExampleClient_PostRequest() {
	c := httpjson.DefaultClient
	// Disable compression if the server doesn't support compressed POST. That's
	// sadly most of them.
	c.Compress = ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message": "Comfortable"}`))
	}))
	defer ts.Close()

	ctx := context.Background()
	h := http.Header{}
	h.Set("Authentication", "Bearer 123")
	in := map[string]string{"question": "weather"}
	resp, err := c.PostRequest(ctx, ts.URL, h, in)
	if err != nil {
		log.Fatal(err)
	}
	// Useful to stream the response. This is also useful if you want to allow
	// unknown fields.
	defer resp.Body.Close()
	out := map[string]string{}
	d := json.NewDecoder(resp.Body)
	d.DisallowUnknownFields()
	d.UseNumber()
	if err := d.Decode(&out); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %s\n", out["message"])
	// Output: Response: Comfortable
}

func init() {
	// slog.SetLogLoggerLevel(slog.LevelDebug)
	slog.SetDefault(slog.New(slog.DiscardHandler))
}
