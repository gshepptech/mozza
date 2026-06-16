// Package ui provides the embedded web dashboard assets.
// Build the UI with: cd ui && npm run build
// The build output goes to internal/ui/dist/ which is embedded at compile time.
package ui

import "embed"

// DistFS contains the built web dashboard files.
// During development, run the UI dev server directly.
// For production, build the UI first and the files will be embedded.
//
//go:embed dist/*
var DistFS embed.FS
