package data

import "embed"

//go:embed catalog.json popularity.json
var Files embed.FS
