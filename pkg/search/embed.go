//go:build darwin

package search

import "embed"

//go:embed rg-darwin-arm64 rg-darwin-amd64
var rgFS embed.FS
