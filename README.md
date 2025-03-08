# httpjson

Magically ✨ handles HTTP JSON requests especially for backends implemented in
dynamic languages (python, ruby, nodejs, etc) that do not have normalized
structured response schema.

Augments the standard library's implementation with compression 🚀 (upload with
gzip) and decompression (gzip, br, zstd). The standard library only support
decompression of gzip.

Enforce no unknown reponse field. Expose function to handle fallback response
schemas, e.g. in case of errors.

Implemented with minimal dependencies.

[![Go Reference](https://pkg.go.dev/badge/github.com/maruel/httpjson/.svg)](https://pkg.go.dev/github.com/maruel/httpjson/)
[![codecov](https://codecov.io/gh/maruel/httpjson/graph/badge.svg?token=EK9DS17M02)](https://codecov.io/gh/maruel/httpjson)
