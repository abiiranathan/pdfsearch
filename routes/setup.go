package routes

import (
	"embed"
	"html/template"
	"net/http"
)

func SetupRoutes(mux *http.ServeMux, staticFs embed.FS, pagesDir string, tmpl *template.Template) {
	// Home path
	mux.HandleFunc("GET /{$}", Home(tmpl))

	// Search endpoint
	mux.HandleFunc("GET /search", Search())

	// Open specific page.
	mux.HandleFunc("GET /books/{book_id}/{page_num}", ServerPage(tmpl, pagesDir))

	// Open books page
	mux.HandleFunc("GET /books", ListBooks(tmpl))

	// Open document with xdg-open if on localhost or serve it
	mux.HandleFunc("GET /open-document/{book_id}", OpenDocument(pagesDir))

	// Serve generated images
	mux.Handle("/pages/", http.StripPrefix("/pages/", http.FileServer(http.Dir(pagesDir))))

	// Server css and JS
	mux.Handle("/static/", http.FileServerFS(staticFs))
}
