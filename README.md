# httpjson

A deceptively simple JSON REST HTTP client. 🧐

Magically ✨ handles HTTP structured JSON requests with especially for backends
implemented in dynamic languages (python, ruby, nodejs, etc) that do not have
normalized structured response schema.

- Enforces no unknown response field by default.
- Exposes functions to gracefully handle fallback response schemas, e.g. in case of errors.
- Supports `context.Context` for cancellation.
- Supports `http.Client` for custom configuration, like recording or custom logging!
  - See sister package
    [roundtrippers](https://pkg.go.dev/github.com/maruel/roundtrippers) for
    implementations.
- Implemented with zero external dependencies.
- Good code coverage.
- Tested on linux, macOS and Windows.
- Works great with the sister package
  [roundtrippers](https://pkg.go.dev/github.com/maruel/roundtrippers/) to add
  logging, authentication, request IDs, compression and more! Works great with
  [go-vcr](https://pkg.go.dev/gopkg.in/dnaeon/go-vcr.v4@v4.0.2/pkg/recorder)
  for unit test recorded HTTP session playback.

[![Go Reference](https://pkg.go.dev/badge/github.com/maruel/httpjson/.svg)](https://pkg.go.dev/github.com/maruel/httpjson/)
[![codecov](https://codecov.io/gh/maruel/httpjson/graph/badge.svg?token=EK9DS17M02)](https://codecov.io/gh/maruel/httpjson)

## Usage

Strictly handle JSON replies.

Try this example in the [Go Playground](https://go.dev/play/p/TZ_vLdkcy0R) ✨

```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/maruel/httpjson"
)

func main() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"message": "Comfortable"}`))
	}))
	defer ts.Close()

	var out struct {
		Message string `json:"message"`
	}
	if err := httpjson.DefaultClient.Get(context.Background(), ts.URL, nil, &out); err != nil {
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
}
```
