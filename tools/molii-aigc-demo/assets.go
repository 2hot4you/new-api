// Package demoassets embeds the standalone test-lab frontend.
package demoassets

import (
	"embed"
	"io/fs"
)

//go:embed static/*
var files embed.FS

func FS() (fs.FS, error) {
	return fs.Sub(files, "static")
}
