package routes

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/abiiranathan/pdfsearch/search"
)

func SetupRoutes(mux *http.ServeMux, staticFs embed.FS,
	pagesDir string, tmpl *template.Template, searchIndex *search.SearchIndex) {
	// Home path
	mux.HandleFunc("GET /{$}", Home(tmpl, searchIndex))

	// Search endpoint
	mux.HandleFunc("GET /search", Search(searchIndex))

	// Open specific page.
	mux.HandleFunc("GET /books/{book_id}/{page_num}", ServerPage(tmpl, pagesDir))

	// Open books page
	mux.HandleFunc("GET /books", ListBooks(tmpl, searchIndex))

	// Open document with xdg-open if on localhost or serve it
	mux.HandleFunc("GET /open-document/{book_id}", OpenDocument(pagesDir))

	// Serve generated images
	mux.Handle("/pages/", http.StripPrefix("/pages/", http.FileServer(http.Dir(pagesDir))))

	// Server css and JS
	mux.Handle("/static/", http.FileServerFS(staticFs))
}
