// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package httpjson_test

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"

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
	_, _ = c.Write(data)
	_ = c.Close()
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
		var herr *httpjson.Error
		if errors.As(err, &herr) {
			fmt.Printf("httpjson.Error: %v", herr)
		}
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
	// Output:
	// Fallback: I only answer weather questions
	// httpjson.Error: http 200
	// {"error":"I only answer weather questions","got":"life"}
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
	_, _ = c.Write(data)
	_ = c.Close()
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
