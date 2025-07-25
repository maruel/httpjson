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
	"reflect"
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
		t.Errorf("Unexpected\nwant: %v\ngot:  %v", "data", out.Output)
	}
}

func TestClient_Get_header(t *testing.T) {
	t.Parallel()
	t.Run("add", func(t *testing.T) {
		t.Parallel()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if h := r.Header.Get("X-Test"); h != "value" {
				t.Errorf("Unexpected\nwant: %v\ngot:  %v", "value", h)
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
		if !errors.As(err, &herr) {
			t.Error("expected Error")
		}
		var jerr *json.SyntaxError
		if !errors.As(err, &jerr) {
			t.Error("expected json.SyntaxError")
		}
		want := "invalid character 'o' in literal null (expecting 'u')\nhttp 200\nnot json"
		if err.Error() != want {
			t.Errorf("failed\nwant: %q\ngot:  %q", want, err)
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
		want := "unknown field *struct { Different string \"json:\\\"different\\\"\" }.output of type string with value \"data\"\nhttp 200\n{\"output\":\"data\"}"
		if got := err.Error(); got != want {
			t.Errorf("unexpected error\nwant: %q\ngot:  %q", want, got)
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
			t.Errorf("Unexpected\nwant: %v\ngot:  %v", "data", in.Input)
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
		t.Errorf("Unexpected\nwant: %v\ngot:  %v", "data", out.Output)
	}
}

func TestClient_Post_error_url(t *testing.T) {
	if err := (&Client{}).Post(context.Background(), "bad\x00url", nil, nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeJSON(t *testing.T) {
	var out struct {
		Output string `json:"output"`
		Extra  string `json:"extra"`
	}
	data := []string{
		`{"output":"data"}`,
		`{"output":"data", "extra":"value"}`,
	}
	for i := range data {
		if err := decodeJSON([]byte(data[i]), &out, false); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDecodeJSON_error(t *testing.T) {
	type NamedNested struct {
		FieldA string
		FieldB int
	}
	type Example struct {
		Name         string `foo:"very bar" json:"name"`
		Age          int    `json:"age,omitempty"`
		Numbers      []int  `json:"numbers,omitzero"`
		Ignored      string `json:"-"`
		Nested       NamedNested
		UnnamedArray []struct {
			FieldC string
		} `json:"unnamed_array"`
	}
	example := reflect.TypeOf(Example{})
	t.Run("root", func(t *testing.T) {
		data := map[string]any{
			"name":    "John",
			"age":     30,
			"numbers": []int{1, 2, 3},
			"Ignored": "unexpected",
		}
		want := []error{&UnknownFieldError{StructType: "httpjson.Example", Field: "Ignored", FieldType: "string", FieldValue: "unexpected"}}
		if got := findExtraKeysGeneric(example, example, data, ""); !errorsEqual(got, want) {
			t.Errorf("Unexpected\nwant: %v\ngot:  %v", want, got)
		}
	})
	t.Run("nested", func(t *testing.T) {
		data := map[string]any{
			"Nested": map[string]any{
				"FieldA": "value",
				"FieldB": 42,
				"Extra2": "unexpected_nested",
			},
		}
		got := findExtraKeysGeneric(example, example, data, "")
		want := []error{&UnknownFieldError{StructType: "httpjson.Example", Field: "Nested.Extra2", FieldType: "string", FieldValue: "unexpected_nested"}}
		if !errorsEqual(got, want) {
			t.Errorf("Unexpected\nwant: %v\ngot:  %v", want, got)
		}
	})
	t.Run("unnamed", func(t *testing.T) {
		data := map[string]any{
			"unnamed_array": []map[string]any{
				{
					"FieldC": "value",
					"Extra3": "unexpected_unnamed",
				},
			},
		}
		got := FindExtraKeys(example, data)
		want := []error{&UnknownFieldError{StructType: "httpjson.Example", Field: "unnamed_array[0].Extra3", FieldType: "string", FieldValue: "unexpected_unnamed"}}
		if !errorsEqual(got, want) {
			t.Errorf("Unexpected\nwant: %v\ngot:  %v", want, got)
		}
	})
}

func TestFindExtraKeys(t *testing.T) {
	type NestedStruct struct {
		ValidField string
	}
	type TestStruct struct {
		Field1 string
		Field2 int
		Nested NestedStruct
	}
	tests := []struct {
		name   string
		t      reflect.Type
		data   any
		prefix string
		want   []error
	}{
		{
			name: "Valid nil",
			t:    reflect.TypeOf([]NestedStruct{}),
			data: nil,
		},
		{
			name: "Valid slice with no extra keys",
			t:    reflect.TypeOf([]NestedStruct{}),
			data: []map[string]any{{"ValidField": "value1"}, {"ValidField": "value2"}},
		},
		{
			name: "Valid slice with no extra keys (ptr)",
			t:    reflect.TypeOf([]*NestedStruct{}),
			data: []map[string]any{{"ValidField": "value1"}, {"ValidField": "value2"}},
		},
		{
			name: "Slice with extra keys",
			t:    reflect.TypeOf([]NestedStruct{}),
			data: []map[string]any{{"ValidField": "value1", "ExtraField": "extra"}},
			// Technically the '.' is incorrect. It comes from reflect.Type.String().
			want: []error{&UnknownFieldError{StructType: "[]httpjson.NestedStruct.[0]", Field: "ExtraField", FieldType: "string", FieldValue: "extra"}},
		},
		{
			name: "Empty slice",
			t:    reflect.TypeOf([]NestedStruct{}),
			data: []map[string]any{},
		},
		{
			name: "Non-slice data",
			t:    reflect.TypeOf([]NestedStruct{}),
			data: "invalid",
			want: []error{&UnknownFieldError{StructType: "[]httpjson.NestedStruct", Field: "", FieldType: "string", FieldValue: "invalid"}},
		},
		{
			name: "Valid map with no extra keys",
			t:    reflect.TypeOf(TestStruct{}),
			data: map[string]any{"Field1": "value1", "Field2": 42, "Nested": map[string]any{"ValidField": "nestedValue"}},
		},
		{
			name: "Inconsistent map",
			t:    reflect.TypeOf(map[string]string{}),
			data: map[string]any{"str": "foo", "int": 42},
		},
		{
			name: "Inconsistent map",
			t:    reflect.TypeOf(map[string]string{}),
			data: []any{"str", 42},
			want: []error{&UnknownFieldError{StructType: "map[string]string", Field: "", FieldType: "[]interface {}", FieldValue: []any{"str", 42}}},
		},
		{
			name: "Map with extra keys at top level",
			t:    reflect.TypeOf(TestStruct{}),
			data: map[string]any{"Field1": "value1", "ExtraField": "extra"},
			want: []error{&UnknownFieldError{StructType: "httpjson.TestStruct", Field: "ExtraField", FieldType: "string", FieldValue: "extra"}},
		},
		{
			name: "Map with extra keys in nested struct",
			t:    reflect.TypeOf(TestStruct{}),
			data: map[string]any{"Field1": "value1", "Nested": map[string]any{"ValidField": "nestedValue", "ExtraNestedField": "extra"}},
			want: []error{&UnknownFieldError{StructType: "httpjson.TestStruct", Field: "Nested.ExtraNestedField", FieldType: "string", FieldValue: "extra"}},
		},
		{
			name: "Empty map",
			t:    reflect.TypeOf(TestStruct{}),
			data: map[string]any{},
		},
		{
			name: "Map with invalid nested type",
			t:    reflect.TypeOf(TestStruct{}),
			data: map[string]any{"Field1": "value1", "Nested": "invalid"},
			want: []error{&UnknownFieldError{StructType: "httpjson.TestStruct", Field: "Nested", FieldType: "string", FieldValue: "invalid"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FindExtraKeys(tt.t, tt.data); !errorsEqual(got, tt.want) {
				t.Errorf("failed\nwant: %v\ngot:  %v", tt.want, got)
			}
		})
	}
}

func errorsEqual(a, b []error) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Error() != b[i].Error() {
			return false
		}
	}
	return true
}
