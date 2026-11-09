// Package testdata embeds the snapshots and fixtures shipped with the binary.
package testdata

import "embed"

// Files contains the pinned server contracts and fake responses.
//
//go:embed openapi-*.json fake/*.json fake/*.pdf fake/*.csv
var Files embed.FS
