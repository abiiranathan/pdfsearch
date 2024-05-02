package routes

import (
	"cmp"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/abiiranathan/pdfsearch/pdf"
	"github.com/abiiranathan/pdfsearch/search"
)

type Book struct {
	ID   uint32
	Name string
	URL  string
}

func Home(tmpl *template.Template, searchIndex *search.SearchIndex) http.HandlerFunc {
	availableBooks := map[uint32]Book{}
	for key := range *searchIndex {
		h := pdf.GetPathHash(key.Filename)
		if _, ok := availableBooks[h]; !ok {
			availableBooks[h] = Book{
				ID:   h,
				Name: filepath.Base(key.Filename),
				URL:  fmt.Sprintf("/open-document/%d", h)}
		}
	}

	books := make([]Book, 0, len(availableBooks))
	for _, book := range availableBooks {
		if !slices.ContainsFunc(books, func(b Book) bool {
			return filepath.Base(book.Name) == filepath.Base(b.Name)
		}) {
			books = append(books, book)
		}
	}

	slices.SortStableFunc(books, func(a, b Book) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return func(w http.ResponseWriter, r *http.Request) {
		higlightMatches := os.Getenv("HIGHLIGHT_MATCHES")
		err := tmpl.ExecuteTemplate(w, "index.html", map[string]any{
			"HighlightEnabled": higlightMatches,
			"books":            books,
		})

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func Search(searchIndex *search.SearchIndex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		book := r.URL.Query().Get("book")

		var bookId []uint32

		if book != "" {
			bookIdInt, err := strconv.Atoi(book)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]string{
					"message": "Invalid book",
				})
				return
			}
			bookId = append(bookId, uint32(bookIdInt))
		}

		if query != "" {
			matches, err := search.Search(query, searchIndex, bookId...)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"message": err.Error(),
				})

				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "max-age=31536000")
			json.NewEncoder(w).Encode(matches)
		} else {
			json.NewEncoder(w).Encode([]string{})

		}
	}
}

func ServerPage(tmpl *template.Template, pagesDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bookID := r.PathValue("book_id")
		pageNum := r.PathValue("page_num")

		bookIDInt, err := strconv.Atoi(bookID)
		if err != nil {
			http.Error(w, "Invalid book id", http.StatusBadRequest)
			return
		}

		pageNumInt, err := strconv.Atoi(pageNum)
		if err != nil {
			http.Error(w, "Invalid page number", http.StatusBadRequest)
			return
		}

		path, valid := pdf.GetPathFromHash(uint32(bookIDInt))
		if !valid {
			http.Error(w, "Invalid book id: Not found in index!", http.StatusBadRequest)
			return
		}

		// Uses a temporary file(os.TempDir) to serve the pdf.
		tempfile, err := os.CreateTemp(pagesDir, "*.pdf")
		if err != nil {
			http.Error(w, "Unable to create temp file", http.StatusInternalServerError)
			return
		}

		// Open document, render image in one single cgo call.
		if !pdf.RenderPageToPDF(pageNumInt, path, tempfile.Name()) {
			http.Error(w, "Unable to generate pdf for the page", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Cache-Control", "max-age=31536000")
		w.Header().Set("Content-Type", "text/html")

		tmpl.ExecuteTemplate(w, "page.html", map[string]any{
			"Title":   filepath.Base(path),
			"URL":     fmt.Sprintf("/%s", tempfile.Name()),
			"ID":      bookID,
			"PrevURL": fmt.Sprintf("/books/%s/%d", bookID, pageNumInt-1),
			"NextURL": fmt.Sprintf("/books/%s/%d", bookID, pageNumInt+1),
		})
	}
}

func OpenDocument(pagesDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bookID := r.PathValue("book_id")

		bookIDInt, err := strconv.Atoi(bookID)
		if err != nil {
			http.Error(w, "Invalid book id", http.StatusBadRequest)
			return
		}

		path, valid := pdf.GetPathFromHash(uint32(bookIDInt))
		if !valid {
			http.Error(w, "Invalid book id: Not found in index!", http.StatusBadRequest)
			return
		}

		// Open the document with xdg-open if on localhost
		host := strings.Split(r.Host, ":")[0]
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			var cmd *exec.Cmd
			if runtime.GOOS == "windows" {
				cmd = exec.Command("cmd", "/c", "start", path)
			} else if runtime.GOOS == "darwin" {
				cmd = exec.Command("open", path)
			} else {
				cmd = exec.Command("xdg-open", path)
			}

			err := cmd.Run()
			if err != nil {
				log.Printf("unable to open %s with default application. Serving it instead\n", path)
				w.Header().Set("Cache-Control", "max-age=31536000")
				http.ServeFile(w, r, path)
				return
			}

			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "<h1>Opening PDF with default application</h1>")
		} else {
			http.ServeFile(w, r, path)
		}
	}
}
