// Package subsonic implements the OpenSubsonic API by wrapping OpenCloud's
// Graph search endpoints and WebDAV streaming.
//
// The ServerInterface, request parameter structs, and response types are
// generated from the upstream OpenSubsonic OpenAPI spec at the commit
// pinned below. The generated file (generated.go) is NOT committed — run
// `make generate` (which executes `go generate ./...`) after checkout or
// whenever the pinned commit is bumped.
//
// Renovate's custom manager in renovate.json5 keeps the commit in sync
// with the upstream main branch by rewriting the SHA in the directive
// below.
package subsonic

// The spec is split across many files (endpoints/, responses/, schemas/)
// via $ref, so we fetch the whole repo tarball at the pinned commit.

//go:generate sh -c "rm -rf .cache && mkdir -p .cache && curl -fsSL https://github.com/opensubsonic/open-subsonic-api/archive/899c511816412a6c2afc7f44e1364535ebc32792.tar.gz | tar -xz -C .cache --strip-components=1"
//go:generate go tool oapi-codegen -package subsonic -generate types,chi-server,spec -o generated.go .cache/openapi/openapi.json
