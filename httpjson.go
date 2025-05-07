// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package httpjson is a deceptively simple JSON REST HTTP client.
package httpjson

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
)

// Client is a JSON REST HTTP client using good default behavior.
type Client struct {
	// Client defaults to http.DefaultClient. Override with http.RoundTripper to
	// add functionality. See github.com/maruel/roundtrippers for useful ones
	// like logging, compression and more!
	Client *http.Client
	// Lenient allows unknown fields in the response.
	//
	// This inhibits from calling DisallowUnknownFields() on the JSON decoder.
	//
	// Use this in production so that your client doesn't break when the server
	// add new fields.
	Lenient bool

	_ struct{}
}

// DefaultClient uses http.DefaultClient and refuses unknown fields.
var DefaultClient = Client{}

// Get simplifies doing an HTTP GET in JSON.
//
// It fails on unknown fields in the response.
// Buffers response body in memory.
func (c *Client) Get(ctx context.Context, url string, hdr http.Header, out any) error {
	resp, err := c.GetRequest(ctx, url, hdr)
	if err != nil {
		return err
	}
	return c.decodeResponse(resp, out)
}

// GetRequest simplifies doing an HTTP POST in JSON.
//
// It is a shorthand for Request().
//
// It initiates the requests and returns the response back for further processing.
func (c *Client) GetRequest(ctx context.Context, url string, hdr http.Header) (*http.Response, error) {
	return c.Request(ctx, "GET", url, hdr, nil)
}

// Post simplifies doing an HTTP POST in JSON.
//
// It fails on unknown fields in the response.
// Buffers both post data and response body in memory.
func (c *Client) Post(ctx context.Context, url string, hdr http.Header, in, out any) error {
	resp, err := c.PostRequest(ctx, url, hdr, in)
	if err != nil {
		return err
	}
	return c.decodeResponse(resp, out)
}

// PostRequest simplifies doing an HTTP POST in JSON.
//
// It initiates the requests and returns the response back for further processing.
// Buffers post data in memory.
func (c *Client) PostRequest(ctx context.Context, url string, hdr http.Header, in any) (*http.Response, error) {
	if in == nil {
		// Catch inattentionnal nil.
		return nil, fmt.Errorf("in is nil")
	}
	return c.Request(ctx, "POST", url, hdr, in)
}

// Request simplifies doing an HTTP PATCH/DELETE/PUT in JSON.
//
// In is optional.
//
// It initiates the requests and returns the response back for further processing.
// Buffers post data in memory.
func (c *Client) Request(ctx context.Context, method, url string, hdr http.Header, in any) (*http.Response, error) {
	var b io.Reader
	if in != nil {
		buf := &bytes.Buffer{}
		e := json.NewEncoder(buf)
		// OMG this took me a while to figure this out. This affects LLM token encoding.
		e.SetEscapeHTML(false)
		if err := e.Encode(in); err != nil {
			return nil, fmt.Errorf("internal error: %w", err)
		}
		b = buf
	}
	req, err := http.NewRequestWithContext(ctx, method, url, b)
	if err != nil {
		return nil, err
	}
	return c.Do(req, hdr)
}

// Do sets the correct headers and allow adding per-request headers.
func (c *Client) Do(req *http.Request, hdr http.Header) (*http.Response, error) {
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	for k, v := range hdr {
		switch len(v) {
		case 0:
			req.Header.Del(k)
		case 1:
			req.Header.Set(k, v[0])
		default:
			for _, vv := range v {
				req.Header.Add(k, vv)
			}
		}
	}
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

// DecodeResponse parses the response body as JSON, trying strict decoding for
// each of the output struct passed in, falling back as the decoding fails. It
// then closes the response body.
//
// Returns the index of which output structured was decoded along joined errors
// for both json decode failure (*json.UnmarshalTypeError, *json.SyntaxError,
// *json.InvalidUnmarshalError) and HTTP status code (*httpjson.Error). Returns
// -1 as the index if no output was decoded.
//
// Buffers response body in memory.
func DecodeResponse(resp *http.Response, out ...any) (int, error) {
	res := -1
	b, err := io.ReadAll(resp.Body)
	if err2 := resp.Body.Close(); err == nil {
		err = err2
	}
	if err != nil {
		return res, fmt.Errorf("failed to read server response: %w", err)
	}
	var errs []error
	for i := range out {
		if err = decodeJSON(b, out[i], false); err == nil {
			res = i
			break
		}
		errs = append(errs, fmt.Errorf("failed to decode server response option #%d: %w", i, err))
	}
	if len(errs) != 0 || resp.StatusCode >= 400 {
		// Include the body in case of error so the user can diagnose.
		errs = append(errs, &Error{ResponseBody: b, StatusCode: resp.StatusCode, Status: resp.Status, PrintBody: len(errs) != 0})
	}
	return res, errors.Join(errs...)
}

func (c *Client) decodeResponse(resp *http.Response, out any) error {
	b, err := io.ReadAll(resp.Body)
	if err2 := resp.Body.Close(); err == nil {
		err = err2
	}
	if err != nil {
		return fmt.Errorf("failed to read server response: %w", err)
	}
	if err = decodeJSON(b, out, c.Lenient); err != nil {
		return errors.Join(err, &Error{ResponseBody: b, StatusCode: resp.StatusCode, Status: resp.Status, PrintBody: true})
	}
	return nil
}

func decodeJSON(b []byte, out any, lenient bool) error {
	d := json.NewDecoder(bytes.NewReader(b))
	if !lenient {
		d.DisallowUnknownFields()
	}
	d.UseNumber()
	if err := d.Decode(out); err != nil {
		if lenient {
			return err
		}
		// decode.object() in encoding/json.go does not return a structured error
		// when an unknown field is found. Process it manually.
		if strings.Contains(err.Error(), "json: unknown field ") {
			// Decode again but this time capture all errors.
			m := map[string]any{}
			d = json.NewDecoder(bytes.NewReader(b))
			d.UseNumber()
			if d.Decode(&m) == nil {
				if err2 := errors.Join(findExtraKeysGeneric(reflect.TypeOf(out), m, "")...); err2 != nil {
					return err2
				}
			}
			return err
		}
		return err
	}
	return nil
}

func findExtraKeysGeneric(t reflect.Type, value any, prefix string) []error {
	if value == nil {
		return nil
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	for {
		if v := reflect.ValueOf(value); v.Kind() == reflect.Ptr {
			value = v.Elem().Interface()
		} else {
			break
		}
	}
	switch t.Kind() {
	case reflect.Struct, reflect.Map:
		if v, ok := value.(map[string]any); ok {
			return findExtraKeysStruct(t, v, prefix)
		}
		return []error{&UnknownFieldError{Field: prefix, Type: fmt.Sprintf("%T", value)}}
	case reflect.Slice, reflect.Array:
		return findExtraKeysSlice(t, value, prefix)
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Uintptr, reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128,
		reflect.String:
		// TODO: Confirm the type.
		return nil
	// case reflect.Chan, reflect.Func, reflect.Interface, reflect.UnsafePointer:
	default:
		return []error{&UnknownFieldError{Field: prefix, Type: fmt.Sprintf("%T", value)}}
	}
}

func findExtraKeysStruct(t reflect.Type, data map[string]any, prefix string) []error {
	validFields := make(map[string]string, t.NumField())
	for i := range t.NumField() {
		// Only consider exported fields.
		if f := t.Field(i); f.PkgPath == "" {
			if jsonName := strings.Split(f.Tag.Get("json"), ",")[0]; jsonName == "" {
				validFields[f.Name] = f.Name
			} else if jsonName != "-" {
				validFields[jsonName] = f.Name
			}
		}
	}
	var out []error
	for key, value := range data {
		v := key
		if prefix != "" {
			v = prefix + "." + key
		}
		if name, ok := validFields[key]; !ok {
			out = append(out, &UnknownFieldError{Field: v, Type: fmt.Sprintf("%T", value)})
		} else if st, ok := t.FieldByName(name); ok {
			out = append(out, findExtraKeysGeneric(st.Type, value, v)...)
		}
	}
	return out
}

func findExtraKeysSlice(t reflect.Type, data any, prefix string) []error {
	d2 := reflect.ValueOf(data)
	if d2.Kind() != reflect.Slice && d2.Kind() != reflect.Array {
		return []error{&UnknownFieldError{Field: prefix, Type: fmt.Sprintf("%T", data)}}
	}
	var out []error
	for i := range d2.Len() {
		out = append(out, findExtraKeysGeneric(t.Elem(), d2.Index(i).Interface(), prefix+fmt.Sprintf("[%d]", i))...)
	}
	return out
}

//

// Error represents an HTTP request that returned an HTTP error.
// It contains the response body if any.
type Error struct {
	ResponseBody []byte
	StatusCode   int
	Status       string
	PrintBody    bool
}

// Error implements error, returning "http <status code>".
func (h *Error) Error() string {
	out := fmt.Sprintf("http %d", h.StatusCode)
	if h.PrintBody {
		out += "\n" + string(h.ResponseBody)
	}
	return out
}

// UnknownFieldError is one unknown field in the JSON response.
type UnknownFieldError struct {
	Field string
	Type  string
}

// Error implements the error interface.
func (e *UnknownFieldError) Error() string {
	return fmt.Sprintf("unknown field %q of type %q", e.Field, e.Type)
}
