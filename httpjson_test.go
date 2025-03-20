// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package httpjson

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

func TestClient_Get_compression(t *testing.T) {
	t.Parallel()
	data := []struct {
		compression string
		handler     http.HandlerFunc
	}{
		{
			"gzip",
			func(w http.ResponseWriter, r *http.Request) {
				if !acceptCompressed(r, "gzip") {
					t.Error("no accept")
				}
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				gz := gzip.NewWriter(w)
				if _, err := gz.Write([]byte(`{"output":"data"}`)); err != nil {
					t.Error(err)
				}
				if err := gz.Close(); err != nil {
					t.Error(err)
				}
			},
		},
		{
			"br",
			func(w http.ResponseWriter, r *http.Request) {
				if !acceptCompressed(r, "br") {
					t.Error("no accept")
				}
				w.Header().Set("Content-Encoding", "br")
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				br := brotli.NewWriter(w)
				if _, err := br.Write([]byte(`{"output":"data"}`)); err != nil {
					t.Error(err)
				}
				if err := br.Close(); err != nil {
					t.Error(err)
				}
			},
		},
		{
			"zstd",
			func(w http.ResponseWriter, r *http.Request) {
				if !acceptCompressed(r, "zstd") {
					t.Error("no accept")
				}
				w.Header().Set("Content-Encoding", "zstd")
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				zs, err := zstd.NewWriter(w)
				if err != nil {
					t.Error(err)
				}
				if _, err = zs.Write([]byte(`{"output":"data"}`)); err != nil {
					t.Error(err)
				}
				if err = zs.Close(); err != nil {
					t.Error(err)
				}
			},
		},
	}
	for _, d := range data {
		t.Run(d.compression, func(t *testing.T) {
			t.Parallel()
			ts := httptest.NewServer(d.handler)
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
		})
	}
}

func acceptCompressed(r *http.Request, want string) bool {
	for encoding := range strings.SplitSeq(r.Header.Get("Accept-Encoding"), ",") {
		if strings.TrimSpace(encoding) == want {
			return true
		}
	}
	return false
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

		c := Client{DefaultHeader: http.Header{"X-Test": []string{"value"}}}
		if err := c.Get(context.Background(), ts.URL, nil, &map[string]string{}); err != nil {
			t.Fatal(err)
		}
		c = Client{}
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

		c := Client{DefaultHeader: http.Header{"X-Test": []string{"value1", "value2"}}}
		if err := c.Get(context.Background(), ts.URL, nil, &map[string]string{}); err != nil {
			t.Fatal(err)
		}
		c = Client{}
		hdr := http.Header{"X-Test": []string{"value1", "value2"}}
		if err := c.Get(context.Background(), ts.URL, hdr, &map[string]string{}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("remove", func(t *testing.T) {
		t.Parallel()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if h := r.Header.Get("X-Test"); h != "" {
				t.Errorf("got %q, want %q", h, "")
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Write([]byte("null"))
		}))
		defer ts.Close()

		c := Client{DefaultHeader: http.Header{"X-Test": []string{"value"}}}
		hdr := http.Header{"X-Test": nil}
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

func TestClient_Post_compression(t *testing.T) {
	t.Parallel()
	data := []struct {
		compression string
		handler     http.HandlerFunc
	}{
		{
			"gzip",
			func(w http.ResponseWriter, r *http.Request) {
				if ce := r.Header.Get("Content-Encoding"); ce != "gzip" {
					t.Errorf("expected gzip, got %q", ce)
				}
				gz, err := gzip.NewReader(r.Body)
				if err != nil {
					t.Error(err)
				}
				var in struct {
					Input string `json:"input"`
				}
				if err = json.NewDecoder(gz).Decode(&in); err != nil {
					t.Error(err)
				}
				if err = gz.Close(); err != nil {
					t.Error(err)
				}
				if in.Input != "data" {
					t.Errorf("got %q, want %q", in.Input, "data")
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				_, _ = w.Write([]byte(`{"output":"data"}`))
			},
		},
		{
			"br",
			func(w http.ResponseWriter, r *http.Request) {
				if ce := r.Header.Get("Content-Encoding"); ce != "br" {
					t.Errorf("expected br, got %q", ce)
				}
				br := brotli.NewReader(r.Body)
				var in struct {
					Input string `json:"input"`
				}
				if err := json.NewDecoder(br).Decode(&in); err != nil {
					t.Error(err)
				}
				if in.Input != "data" {
					t.Errorf("got %q, want %q", in.Input, "data")
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				_, _ = w.Write([]byte(`{"output":"data"}`))
			},
		},
		{
			"zstd",
			func(w http.ResponseWriter, r *http.Request) {
				if ce := r.Header.Get("Content-Encoding"); ce != "zstd" {
					t.Errorf("expected zstd, got %q", ce)
				}
				zs, err := zstd.NewReader(r.Body)
				if err != nil {
					t.Error(err)
				}
				var in struct {
					Input string `json:"input"`
				}
				if err = json.NewDecoder(zs).Decode(&in); err != nil {
					t.Error(err)
				}
				zs.Close()
				if in.Input != "data" {
					t.Errorf("got %q, want %q", in.Input, "data")
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				_, _ = w.Write([]byte(`{"output":"data"}`))
			},
		},
	}
	for _, d := range data {
		t.Run(d.compression, func(t *testing.T) {
			t.Parallel()
			ts := httptest.NewServer(d.handler)
			defer ts.Close()
			in := map[string]string{"input": "data"}
			var out struct {
				Output string `json:"output"`
			}
			c := Client{PostCompress: d.compression}
			err := c.Post(context.Background(), ts.URL, nil, in, &out)
			if err != nil {
				t.Fatal(err)
			}
			if out.Output != "data" {
				t.Errorf("got %q, want %q", out.Output, "data")
			}
		})
	}
}

func TestClient_Post_error_url(t *testing.T) {
	if err := (&Client{}).Post(context.Background(), "bad\x00url", nil, nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_Post_error_compress(t *testing.T) {
	if err := (&Client{PostCompress: "bad"}).Post(context.Background(), "http://127.0.0.1:0", nil, nil, nil); err == nil {
		t.Fatal("expected error")
	}
}
