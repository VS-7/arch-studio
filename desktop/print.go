package main

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Impressão (PDF) no app desktop.
//
// Webviews não imprimem de forma confiável um documento dentro de um iframe
// (no macOS, window.print() nem existe). Aqui o documento abre numa janela
// própria, servida pelo mesmo asset server — para que as figuras em /api/…
// carreguem — e, quando a página avisa que terminou de carregar, o Go chama
// Print() nessa janela: o diálogo nativo imprime exatamente o documento.

const printPrefix = "/__print/"

type printJob struct {
	html   string
	window *application.WebviewWindow
}

type printJobs struct {
	mu   sync.Mutex
	jobs map[string]*printJob
}

func newPrintJobs() *printJobs { return &printJobs{jobs: map[string]*printJob{}} }

// readyScript avisa o Go quando a página e todas as imagens carregaram.
const readyScript = `<script>
addEventListener('load', function () {
  var pending = Array.prototype.filter.call(document.images, function (i) { return !i.complete })
  Promise.all(pending.map(function (i) { return new Promise(function (r) { i.onload = i.onerror = r }) }))
    .then(function () { fetch(location.pathname + '/ready', { method: 'POST' }) })
})
</script>`

// Print abre a janela de impressão com o documento HTML completo.
func (p *printJobs) Print(app *application.App, title, html string) {
	id := randomID()
	job := &printJob{html: html}
	p.mu.Lock()
	p.jobs[id] = job
	p.mu.Unlock()

	job.window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Imprimir — " + title,
		Width:  900,
		Height: 1000,
		URL:    printPrefix + id,
	})
}

func (p *printJobs) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, printPrefix)
	id, action, _ := strings.Cut(rest, "/")
	ready := action == "ready" && r.Method == http.MethodPost
	p.mu.Lock()
	job := p.jobs[id]
	if ready {
		delete(p.jobs, id) // o documento já carregou; a janela não precisa mais dele
	}
	p.mu.Unlock()
	if job == nil || (action != "" && !ready) {
		http.NotFound(w, r)
		return
	}

	if ready {
		w.WriteHeader(http.StatusNoContent)
		if job.window != nil {
			go func() { _ = job.window.Print() }()
		}
		return
	}
	html := job.html
	if i := strings.LastIndex(strings.ToLower(html), "</body>"); i >= 0 {
		html = html[:i] + readyScript + html[i:]
	} else {
		html += readyScript
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(html))
}

func randomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
