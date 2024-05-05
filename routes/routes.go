package routes

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/abiiranathan/pdfsearch/database"
	"github.com/abiiranathan/pdfsearch/pdf"
)

type Book struct {
	ID   int
	Name string
	URL  string
}

func Home(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := database.GetFiles(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		books := make([]Book, len(files))
		for i, file := range files {
			books[i] = Book{
				ID:   file.ID,
				Name: file.Name,
				URL:  fmt.Sprintf("/open-document/%d", file.ID),
			}
		}

		tmpl.ExecuteTemplate(w, "index.html", map[string]any{
			"books": books,
		})

	}
}

func ListBooks(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := database.GetFiles(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		books := make([]Book, len(files))
		for i, file := range files {
			books[i] = Book{
				ID:   file.ID,
				Name: file.Name,
				URL:  fmt.Sprintf("/open-document/%d", file.ID),
			}
		}

		err = tmpl.ExecuteTemplate(w, "books.html", map[string]any{
			"books": books,
		})

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func Search() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		book := r.URL.Query().Get("book")

		var books []int

		if book != "" {
			bookIdInt, err := strconv.Atoi(book)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]string{
					"message": "Invalid book",
				})
				return
			}
			books = append(books, bookIdInt)
		}

		if query != "" {
			matches, err := database.Search(r.Context(), query, books...)
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

		file, err := database.GetFile(r.Context(), bookIDInt)
		if err != nil {
			http.Error(w, "Unable to get file", http.StatusNotFound)
			return
		}

		doc := pdf.Open(file.Path)
		if doc == nil {
			http.Error(w, "Unable to open document", http.StatusInternalServerError)
			return
		}
		defer doc.Close()

		// Uses a temporary file(os.TempDir) to serve the pdf.
		tempfile, err := os.CreateTemp(pagesDir, "*.pdf")
		if err != nil {
			http.Error(w, "Unable to create temp file", http.StatusInternalServerError)
			return
		}

		// Open document, render image in one single cgo call.
		if !pdf.RenderPageToPDF(pageNumInt, file.Path, tempfile.Name()) {
			http.Error(w, "Unable to generate pdf for the page", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Cache-Control", "max-age=31536000")
		w.Header().Set("Content-Type", "text/html")

		data := map[string]any{
			"Title": file.Name,
			"URL":   fmt.Sprintf("/%s", tempfile.Name()),
			"ID":    bookID,
		}

		if pageNumInt != 0 {
			data["FirstURL"] = fmt.Sprintf("/books/%s/0", bookID)
		}

		if pageNumInt != doc.NumPages-1 {
			data["LastURL"] = fmt.Sprintf("/books/%s/%d", bookID, doc.NumPages-1)
		}

		if pageNumInt > 0 {
			data["PrevURL"] = fmt.Sprintf("/books/%s/%d", bookID, pageNumInt-1)
		}

		if pageNumInt < doc.NumPages-1 {
			data["NextURL"] = fmt.Sprintf("/books/%s/%d", bookID, pageNumInt+1)
		}

		tmpl.ExecuteTemplate(w, "page.html", data)
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

		file, err := database.GetFile(r.Context(), bookIDInt)
		if err != nil {
			http.Error(w, "Unable to get file", http.StatusNotFound)
			return
		}

		// Serve the file if it exists
		path := file.Path

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
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		} else {
			http.ServeFile(w, r, path)
		}
	}
}
