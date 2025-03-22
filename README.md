# httpjson

A deceptively simple JSON REST HTTP client. 🧐

Magically ✨ handles HTTP structured JSON requests with especially for backends
implemented in dynamic languages (python, ruby, nodejs, etc) that do not have
normalized structured response schema.

- Augments the standard library's implementation with upload compression 🚀 and
  add [brotli](https://caniuse.com/?search=brotli) and
  [zstd](https://caniuse.com/?search=zstd) decompression. The standard library
  only support decompression of [gzip](https://caniuse.com/?search=gzip).
- Enforces no unknown response field by default.
- Exposes functions to gracefully handle fallback response schemas, e.g. in case of errors.
- Supports `context.Context` for cancellation.
- Supports `http.Client` for custom configuration, like recording or custom logging!
  - See sister package
    [roundtrippers](https://pkg.go.dev/github.com/maruel/roundtrippers) for
    implementations.
- Implemented with minimal dependencies.
- Good code coverage.
- Tested on linux, macOS and Windows.
- Works great with the sister package
  [roundtrippers](https://pkg.go.dev/github.com/maruel/roundtrippers/) to add
  logging, authentication, request IDs, compression and more! Works great with
  [go-vcr](https://pkg.go.dev/gopkg.in/dnaeon/go-vcr.v4@v4.0.2/pkg/recorder)
  for unit test recorded HTTP session playback.

[![Go Reference](https://pkg.go.dev/badge/github.com/maruel/httpjson/.svg)](https://pkg.go.dev/github.com/maruel/httpjson/)
[![codecov](https://codecov.io/gh/maruel/httpjson/graph/badge.svg?token=EK9DS17M02)](https://codecov.io/gh/maruel/httpjson)
