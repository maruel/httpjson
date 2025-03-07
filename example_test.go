// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package httpjson_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/maruel/httpjson"
)

func Example_get() {
	c := httpjson.DefaultClient
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"message": "Comfortable"}`)
	}))
	defer ts.Close()

	ctx := context.Background()
	out := map[string]string{}
	if err := c.Get(ctx, ts.URL, nil, &out); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %s\n", out["message"])
	// Output: Response: Comfortable
}

func Example_post() {
	c := httpjson.DefaultClient
	// Disable compression if the server doesn't support compressed POST. That's
	// sadly most of them.
	c.Compress = ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"message": "Comfortable"}`)
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

func Example_postRequest() {
	c := httpjson.DefaultClient
	// Disable compression if the server doesn't support compressed POST. That's
	// sadly most of them.
	c.Compress = ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"message": "Comfortable"}`)
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
