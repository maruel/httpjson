# httpjson

Reduces repetitive code for making HTTP requests and parsing JSON responses!

Augment the standard library's implementation with compression (upload with
gzip) and decompression (gzip, br, zstd). The standard library only support
decompression of gzip.

Defaults to fail on unknown field and use number instead of float64.

Implemented with minimal dependencies.

[![Go Reference](https://pkg.go.dev/badge/github.com/maruel/httpjson/.svg)](https://pkg.go.dev/github.com/maruel/httpjson/)
[![codecov](https://codecov.io/gh/maruel/ask/graph/badge.svg?token=EK9DS17M02)](https://codecov.io/gh/maruel/ask)
