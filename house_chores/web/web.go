// Package web osadza w binarce szablony HTML i pliki statyczne.
package web

import "embed"

//go:embed templates/*.html static/*
var FS embed.FS
