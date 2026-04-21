// Package model holds the oapi-codegen output for the OpenSubsonic API.
//
// It's split off from the parent `subsonic` package so that non-handler
// code (the response-envelope writer in subsonic/proto, the auth
// middleware that emits failure envelopes) can import the generated
// types without recreating the import cycle those packages exist to
// avoid.
//
// The file `generated.go` is NOT committed — run `make backend-generate`
// (which executes `go generate ./...`) after checkout or whenever the
// commit pin below is bumped. Renovate's custom manager in
// renovate.json5 keeps the commit in sync with upstream main.
package model

// The spec is split across many files (endpoints/, responses/, schemas/)
// via $ref, so we fetch the whole repo tarball at the pinned commit.

//go:generate sh -c "rm -rf .cache && mkdir -p .cache && curl -fsSL https://github.com/opensubsonic/open-subsonic-api/archive/899c511816412a6c2afc7f44e1364535ebc32792.tar.gz | tar -xz -C .cache --strip-components=1"

// The OpenSubsonic spec uses relative `$ref`s across sibling
// directories (schemas/, responses/, endpoints/*/); oapi-codegen's
// loader can't follow them and silently drops every schema that hides
// behind such a ref. Bundle the multi-file spec into a single
// self-contained JSON first so the generator sees named component
// types.
//
//go:generate go run ../../tools/bundle-openapi .cache/openapi/openapi.json .cache/openapi-bundled.json
//go:generate go tool oapi-codegen -package model -generate types,chi-server,spec -o generated.go .cache/openapi-bundled.json
