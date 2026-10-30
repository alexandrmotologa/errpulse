package web

import "embed"

// DistFS contains the compiled production single page application assets.
//
//go:embed all:dist
var DistFS embed.FS
