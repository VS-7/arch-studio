// Package webui embute os assets compilados do frontend no binário final,
// eliminando a necessidade de Node.js na máquina do usuário (RNF002).
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// FS devolve a raiz dos assets compilados.
func FS() (fs.FS, error) { return fs.Sub(embedded, "dist") }

// Built informa se o frontend foi compilado neste binário.
func Built() bool {
	f, err := FS()
	if err != nil {
		return false
	}
	if _, err := fs.Stat(f, "index.html"); err != nil {
		return false
	}
	entries, err := fs.ReadDir(f, ".")
	if err != nil {
		return false
	}
	// Um dist real traz ao menos o index e a pasta de assets.
	return len(entries) > 1
}
