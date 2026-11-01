package web

import (
	"embed"
	"io/fs"
)

//go:embed all:build
var build embed.FS

func Assets() fs.FS {
	sub, err := fs.Sub(build, "build")
	if err != nil {
		panic(err)
	}
	return sub
}
