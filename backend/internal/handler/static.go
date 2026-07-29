package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SPA serve il frontend compilato (adapter-static, fallback su index.html).
//
// Sta nello stesso binario dell'API apposta: un'origine sola vuol dire niente
// CORS da configurare in produzione, niente hostname del backend compilato nel
// bundle, e un redirect_uri OAuth che coincide con il dominio pubblico.
//
// Il fallback vale solo per le rotte non-/api: una GET su un endpoint API
// inesistente deve restare un 404 JSON, non l'index.html — altrimenti il client
// si ritrova a fare JSON.parse('<!doctype html>') e l'errore vero sparisce.
func SPA(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			jsonError(w, ErrNotFound, http.StatusNotFound)
			return
		}

		clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		// Clean lascia passare "..": senza questo controllo un path tipo
		// /../../etc/passwd uscirebbe dalla cartella servita.
		if clean == ".." || strings.HasPrefix(clean, "../") {
			http.NotFound(w, r)
			return
		}

		full := filepath.Join(dir, clean)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			// Gli asset di Vite hanno l'hash nel nome: immutabili per sempre.
			// index.html invece no — se lo si cachasse, dopo un deploy l'utente
			// resterebbe sullo shell vecchio che punta a bundle non più esistenti.
			if strings.HasPrefix(clean, "_app/immutable/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}

		// Rotta del client (/train, /plan, …): la risolve il router in browser.
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}
