// Copyright 2025 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package roundtrippers

import (
	"log/slog"
	"net/http"
	"testing"
)

func TestLog_RoundTrip(t *testing.T) {
	c := http.Client{Transport: &Log{Transport: http.DefaultTransport, L: slog.New(slog.DiscardHandler)}}
	resp, err := c.Get("")
	if resp != nil || err == nil {
		t.Fatal(resp, err)
	}
}

func TestCapture_RoundTrip(t *testing.T) {
	ch := make(chan Record, 1)
	c := http.Client{Transport: &Capture{Transport: http.DefaultTransport, C: ch}}
	resp, err := c.Get("")
	if resp != nil || err == nil {
		t.Fatal(resp, err)
	}
}
