// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package httpjson

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Get(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"output":"data"}`))
	}))
	defer ts.Close()
	var out struct {
		Output string `json:"output"`
	}
	c := Client{}
	err := c.Get(context.Background(), ts.URL, nil, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.Output != "data" {
		t.Errorf("got %q, want %q", out.Output, "data")
	}
}

func TestClient_Get_header(t *testing.T) {
	t.Parallel()
	t.Run("add", func(t *testing.T) {
		t.Parallel()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if h := r.Header.Get("X-Test"); h != "value" {
				t.Errorf("got %q, want %q", h, "value")
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Write([]byte("null"))
		}))
		defer ts.Close()

		c := Client{}
		hdr := http.Header{"X-Test": []string{"value"}}
		if err := c.Get(context.Background(), ts.URL, hdr, &map[string]string{}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("multiple", func(t *testing.T) {
		t.Parallel()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if v := r.Header.Values("X-Test"); len(v) != 2 || v[0] != "value1" || v[1] != "value2" {
				t.Errorf("got %q", v)
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Write([]byte("null"))
		}))
		defer ts.Close()

		c := Client{}
		hdr := http.Header{"X-Test": []string{"value1", "value2"}}
		if err := c.Get(context.Background(), ts.URL, hdr, &map[string]string{}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestClient_Get_error_url(t *testing.T) {
	if err := (&Client{}).Get(context.Background(), "bad\x00url", nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_Get_error_bad_decode(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`not json`))
	}))
	defer ts.Close()
	var out struct {
		Output string `json:"output"`
	}
	c := Client{}
	err := c.Get(context.Background(), ts.URL, nil, &out)
	if err != nil {
		var herr *Error
		if errors.As(err, &herr) {
			t.Error("unexpected Error")
		}
		var jerr *json.SyntaxError
		if !errors.As(err, &jerr) {
			t.Error("expected json.SyntaxError")
		}
		if err.Error() != "invalid character 'o' in literal null (expecting 'u')" {
			t.Error("got", err)
		}
	}
}

func TestClient_Get_error_decode_unexpected_field(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"output":"data"}`))
	}))
	defer ts.Close()

	var out struct {
		Different string `json:"different"`
	}
	c := Client{}
	if err := c.Get(context.Background(), ts.URL, nil, &out); err != nil {
		var herr *json.SyntaxError
		if errors.As(err, &herr) {
			t.Error("unexpected Error", herr)
		}
		var jerr *json.SyntaxError
		if errors.As(err, &jerr) {
			t.Error("unexpected json.SyntaxError", jerr)
		}
		if err.Error() != `json: unknown field "output"` {
			t.Error("got", err)
		}
	}

	// Same but with lenient.
	c.Lenient = true
	if err := c.Get(context.Background(), ts.URL, nil, &out); err != nil {
		t.Error(err)
	}
}

//

func TestClient_Post(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Input string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Error(err)
		}
		if in.Input != "data" {
			t.Errorf("got %q, want %q", in.Input, "data")
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"output":"data"}`))
	}))
	defer ts.Close()
	in := map[string]string{"input": "data"}
	var out struct {
		Output string `json:"output"`
	}
	c := Client{}
	err := c.Post(context.Background(), ts.URL, nil, in, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.Output != "data" {
		t.Errorf("got %q, want %q", out.Output, "data")
	}
}

func TestClient_Post_error_url(t *testing.T) {
	if err := (&Client{}).Post(context.Background(), "bad\x00url", nil, nil, nil); err == nil {
		t.Fatal("expected error")
	}
}
