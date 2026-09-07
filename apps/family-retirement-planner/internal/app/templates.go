package app

import (
	"embed"
	"html/template"
	"io/fs"
)

//go:embed templates/*.html static/*
var Assets embed.FS

func StaticAssets() fs.FS {
	static, err := fs.Sub(Assets, "static")
	if err != nil {
		panic(err)
	}
	return static
}

func Templates() *template.Template {
	return template.Must(template.ParseFS(Assets, "templates/*.html"))
}
