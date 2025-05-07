// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package httpjson_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/maruel/httpjson"
)

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

func handleGet(w http.ResponseWriter, r *http.Request) {
	out := serverRespondQuestion(r.URL.Query().Get("question"))
	data, err := json.Marshal(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(data)
}

type authorized struct {
	transport http.RoundTripper
	apiKey    string
}

func (a *authorized) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "bearer "+a.apiKey)
	return a.transport.RoundTrip(req)
}

func ExampleClient_custom_transport() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "bearer secret-key" {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"message": "working"}`))
	}))
	defer ts.Close()

	var out struct {
		Message string `json:"message"`
	}
	c := httpjson.Client{
		// This enables using a custom transport that adds HTTP headers automatically.
		Client: &http.Client{
			Transport: &authorized{transport: http.DefaultTransport, apiKey: "secret-key"},
		},
	}
	if err := c.Get(context.Background(), ts.URL, nil, &out); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Response: %s\n", out.Message)
	// Output: Response: working
}

func ExampleClient_lenient() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"message": "working", "new_field":true}`))
	}))
	defer ts.Close()

	var out struct {
		Message string `json:"message"`
	}
	c := httpjson.Client{
		// This enables the decoding to work even if unexpected fields are returned.
		Lenient: true,
	}
	if err := c.Get(context.Background(), ts.URL, nil, &out); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Response: %s\n", out.Message)
	// Output: Response: working
}

func ExampleClient_Get() {
	ts := httptest.NewServer(http.HandlerFunc(handleGet))
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
	ts := httptest.NewServer(http.HandlerFunc(handleGet))
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

func handlePost(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Question string `json:"question"`
	}
	var out any
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		out = map[string]string{"error": err.Error()}
	} else {
		out = serverRespondQuestion(in.Question)
	}
	data, err := json.Marshal(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(data)
}

func ExampleClient_Post() {
	ts := httptest.NewServer(http.HandlerFunc(handlePost))
	defer ts.Close()

	ctx := context.Background()
	h := http.Header{}
	h.Set("Authentication", "Bearer 123")
	in := map[string]string{"question": "weather"}
	out := map[string]string{}
	if err := httpjson.DefaultClient.Post(ctx, ts.URL, h, in, &out); err != nil {
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

func ExampleClient_Request() {
	// Use Request() to send a DELETE request.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		data, err := json.Marshal(map[string]string{"message": "done"})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	resp, err := httpjson.DefaultClient.Request(context.Background(), "DELETE", ts.URL, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	out := map[string]string{}
	i, err := httpjson.DecodeResponse(resp, &out)
	switch i {
	case 0:
		fmt.Printf("Success case: %s\n", out["message"])
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
	// Success case: done
}
