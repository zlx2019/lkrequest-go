# lkrequest-go

`lkrequest-go` provides Go bindings for the `lkrequest-ffi` Rust HTTP client library.

It exposes a thin, idiomatic Go API around the FFI surface:

- `Client` for immutable shared client configuration
- `Session` for pools, proxy policy, redirects, and retry behavior
- `Request` for single-owner request construction and sending
- `Response` and `StreamingResponse` for buffered and streaming reads

## Installation

```bash
go get github.com/lkrequest/lkrequest-go
```

Import the package with:

```go
import "github.com/lkrequest/lkrequest-go/lkrequest"
```

## Quick Start

```go
package main

import (
	"fmt"
	"log"

	"github.com/lkrequest/lkrequest-go/lkrequest"
)

func main() {
	resp, err := lkrequest.Get("https://httpbin.org/get")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Close()

	fmt.Println(resp.StatusCode())
	fmt.Println(resp.String())
}
```

## Dual Engine

The package supports two binding engines:

- Default `purego` mode: dynamically loads an embedded shared library and requires no C compiler.
- Optional `lkcgo` mode: links through CGo for users who want explicit static-link style integration.

## API Overview

- Version and capability helpers: `ABIVersion`, `LibraryVersion`, `FeatureSupported`
- Top-level sugar: `Get`, `PostJSON`
- Core types: `Client`, `Session`, `Request`, `Response`, `StreamingResponse`
- Support helpers: `ListPresetsJSON`, `GetPresetDetailJSON`, `InitLog`
- Advanced helpers: `InitLogCallback`, `DisableRedirects`, `SetProvider`, `SetDNSResolver`
- QUIC/H3 helpers: `FeatureSupported("quic-h3")`, `SetHTTP3Only`, `SetHTTP3WithFallback`, `AddH3HeaderOrder`
- QUIC tuning on `ClientBuilder`: `DisableHTTP3`, `SetTimeoutQUICConnect`, `SetQUICProfileJSON`, `SetSessionResumptionJSON`
- Per-request protocol preference: `Request.SetPreferredHTTPVersion`; transport version via `Response.NegotiatedVersion`
- SOCKS5 UDP relay probing: `Client.Socks5UDPProbe`, `Client.Socks5UDPProbeAsync`

## Build Tags

Build with the default embedded-library engine:

```bash
go build ./lkrequest/...
```

Build with the CGo engine:

```bash
go build -tags lkcgo ./lkrequest/...
```

## Notes

- `Request` is consume-on-send. Reuse it through `Clone()`.
- `Client` and `Session` are safe to share across goroutines.
- `StreamingResponse` implements `io.ReadCloser`.
- Upstream `lkrequest` change: `ClientBuilder.Build()` no longer applies an implicit Chrome 144 `header_order` preset. Set a fingerprint or call `AddHeaderOrder` / `AddCookieOrder` on the client, session, or individual request when you need that behavior.
- QUIC/H3 support can be gated with `FeatureSupported("quic-h3")`. H3-specific header order can be configured on `ClientBuilder`, `SessionBuilder`, and `Request`.
- The QUIC/H3, custom-DNS-resolver, SOCKS5 UDP probe, preferred/negotiated HTTP version, and split cookie/redirect accessors require an lkrequest library that exports the corresponding symbols. Against an older embedded library these methods return an `lk: ... not supported by the loaded lkrequest library` error (builders defer it to `Build()`); the package still loads normally.
