package pdf

/*
#cgo pkg-config: glib-2.0 gio-2.0 cairo poppler-glib
#cgo LDFLAGS: -pthread

#include <cairo/cairo.h>
#include <cairo/cairo-pdf.h>
#include <locale.h>
#include <poppler/glib/poppler.h>
#include <stdio.h>
#include <stdbool.h>

static pthread_mutex_t cairo_mutex = PTHREAD_MUTEX_INITIALIZER;

PopplerDocument *open_document(const char *filename, int *num_pages){
	GFile* file = g_file_new_for_path(filename);
	if(file == NULL){
		return NULL;
	}

	GError* error = NULL;
	GBytes* bytes = g_file_load_bytes(file, NULL, NULL, &error);
	g_object_unref(file);

	if (error != NULL) {
		g_print("Error loading PDF file: %s\n", error->message);
		g_clear_error(&error);
		return NULL;
	}

	PopplerDocument *doc = poppler_document_new_from_bytes(bytes, NULL, &error);
	if (error) {
		g_print("Error creating PDF document: %s\n", error->message);
		g_clear_error(&error);
		g_bytes_unref(bytes);
		return NULL;
	}

	*num_pages = poppler_document_get_n_pages(doc);
	g_bytes_unref(bytes);
	return doc;
}

void render_page_to_image(PopplerPage *page, int width, int height, const char* output_file) {
  // Set the desired resolution (300 DPI)
  double resolution = 300.0;
  int pixel_width = (int)(width * resolution / 72.0);
  int pixel_height = (int)(height * resolution / 72.0);

  // Lock the mutex before creating the Cairo surface
  pthread_mutex_lock(&cairo_mutex);

  // Create the Cairo surface with the specified resolution
  cairo_surface_t* surface =
    cairo_image_surface_create(CAIRO_FORMAT_ARGB32, pixel_width, pixel_height);
  if (surface == NULL) {
    pthread_mutex_unlock(&cairo_mutex);
    puts("Unable to create cairo surface");
    return;
  }

  cairo_t* cr = cairo_create(surface);
  if (cr == NULL) {
    pthread_mutex_unlock(&cairo_mutex);
    puts("Error: could not create cairo context");
    return;
  }

  // Set the background color to white (or any desired color)
  cairo_set_source_rgb(cr, 1.0, 1.0, 1.0);  // White background
  cairo_paint(cr);                          // Fill the surface with the background color

  // Disable anti-aliasing for text rendering to avoid blurriness.
  cairo_set_antialias(cr, CAIRO_ANTIALIAS_NONE);

  // Calculate the scaling factors to maintain the original aspect ratio
  double scale_x = pixel_width / width;
  double scale_y = pixel_height / height;
  cairo_scale(cr, scale_x, scale_y);

  poppler_page_render(page, cr);

  // Unlock the mutex after rendering the page
  pthread_mutex_unlock(&cairo_mutex);

  cairo_status_t status = cairo_surface_write_to_png(surface, output_file);
  if (status != CAIRO_STATUS_SUCCESS) {
    puts("Error: could not write to png file");
    return;
  }

  cairo_surface_destroy(surface);
  cairo_destroy(cr);
}

// Render a single page from a document. Avoids multiple cgo calls
bool render_page_from_document(const char *pdf_path, int page_num, const char* output_png){
	int num_pages=0;
	PopplerDocument *doc = open_document(pdf_path, &num_pages);
	if(doc == NULL){
		puts("Error opening document");
		return false;
	}

	if (page_num < 0 || page_num >= num_pages){
		puts("Page number is out of range of this document");
		g_object_unref(doc);
		return false;
	}

	PopplerPage *page = poppler_document_get_page(doc, page_num);
	if(page == NULL){
		printf("PopplerPage for page %d is NULL\n", page_num);
		g_object_unref(doc);
		return false;
	}

	double width, height;
	poppler_page_get_size(page, &width, &height);

	render_page_to_image(page, width, height, output_png);
	g_object_unref(doc);
	g_object_unref(page);
	return true;
}

// Render a pdf of a Poppler Page with cairo
bool poppler_page_to_pdf(PopplerPage *page,  const char* output_pdf){
	cairo_surface_t *surface;
	cairo_t *cr;

	double width, height;
	poppler_page_get_size(page, &width, &height);

	double resolution = 300.0;
	int pixel_width = (int)(width * resolution / 72.0);
	int pixel_height = (int)(height * resolution / 72.0);

	pthread_mutex_lock(&cairo_mutex);

	surface = cairo_pdf_surface_create(output_pdf, pixel_width, pixel_height);
	if (cairo_surface_status(surface) != CAIRO_STATUS_SUCCESS) {
		puts("Error creating PDF surface");
		pthread_mutex_unlock(&cairo_mutex);
		return false;
	}

	cr = cairo_create(surface);
	if (cairo_status(cr) != CAIRO_STATUS_SUCCESS) {
		puts("Error creating cairo context");
		pthread_mutex_unlock(&cairo_mutex);
		return false;
	}

	// Set the background color (optional)
    cairo_set_source_rgb(cr, 1, 1, 1);
    cairo_paint(cr);

	cairo_scale(cr, pixel_width / width, pixel_height / height);

	poppler_page_render(page, cr);

	cairo_surface_finish(surface);

	cairo_destroy(cr);
	cairo_surface_destroy(surface);
	pthread_mutex_unlock(&cairo_mutex);

	return true;
}


// Takes in the page num, pdf path, outpdf path and renders the page to a pdf
bool render_page_to_pdf(const char *pdf_path, int page_num, const char* output_pdf){
	int num_pages=0;
	PopplerDocument *doc = open_document(pdf_path, &num_pages);
	if(doc == NULL){
		puts("Error opening document");
		return false;
	}

	if (page_num < 0 || page_num >= num_pages){
		puts("Page number is out of range of this document");
		g_object_unref(doc);
		return false;
	}

	PopplerPage *page = poppler_document_get_page(doc, page_num);
	if(page == NULL){
		printf("PopplerPage for page %d is NULL\n", page_num);
		g_object_unref(doc);
		return false;
	}

	bool status = poppler_page_to_pdf(page, output_pdf);
	g_object_unref(doc);
	g_object_unref(page);
	return status;
}

*/
import "C"
import (
	"context"
	"encoding/gob"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sync/errgroup"
)

type Document struct {
	doc      *C.PopplerDocument
	Path     string
	NumPages int
}

func SetLocale() {
	// Set locale to UTF-8
	C.setlocale(C.LC_ALL, C.CString(""))
}

func Open(path string) *Document {
	var c_path *C.char = C.CString(path)
	defer C.free(unsafe.Pointer(c_path))

	var num_pages C.int
	pdf := &Document{
		doc:      C.open_document(c_path, &num_pages),
		NumPages: int(num_pages),
		Path:     path,
	}
	return pdf
}

func (pdf *Document) Close() {
	if pdf.doc != nil {
		C.g_object_unref(C.gpointer(pdf.doc))
	}
}

type Page struct {
	page *C.PopplerPage

	doc     *Document
	PageNum int

	Width  float64
	Height float64
}

func (pdf *Document) GetPage(page int) *Page {
	if page < 0 || page >= pdf.NumPages {
		return nil
	}

	p_page := &Page{
		doc:     pdf,
		page:    C.poppler_document_get_page(pdf.doc, C.int(page)),
		PageNum: page,
	}

	var width, height C.double
	C.poppler_page_get_size(p_page.page, &width, &height)
	p_page.Width = float64(width)
	p_page.Height = float64(height)

	return p_page
}

func (page *Page) Close() {
	if page.page != nil {
		C.g_object_unref(C.gpointer(page.page))
	}
}

func (page *Page) Render(output string) {
	c_output := C.CString(output)
	defer C.free(unsafe.Pointer(c_output))

	C.render_page_to_image(page.page, C.int(page.Width), C.int(page.Height), c_output)
}

// Render the page to a PDF file.
func (page *Page) ToPDF(output string) bool {
	c_output := C.CString(output)
	defer C.free(unsafe.Pointer(c_output))

	cbool := C.poppler_page_to_pdf(page.page, c_output)
	return bool(cbool)
}

// Render a pdf page to a PDF file in a single cgo call.
func RenderPageToPDF(pageNum int, pdfPath, outPdf string) bool {
	c_output := C.CString(outPdf)
	defer C.free(unsafe.Pointer(c_output))

	var c_path *C.char = C.CString(pdfPath)
	defer C.free(unsafe.Pointer(c_path))

	cbool := C.render_page_to_pdf(c_path, C.int(pageNum), c_output)
	return bool(cbool)
}

// Render a pdf page to a PNG image in a single cgo call.
// Faster that opening the document, getting a page and calling page.Render().
// Returns true if the image was rendered successfully.
func RenderPageToImage(pageNum int, pdfPath, outPng string) bool {
	c_output := C.CString(outPng)
	defer C.free(unsafe.Pointer(c_output))

	var c_path *C.char = C.CString(pdfPath)
	defer C.free(unsafe.Pointer(c_path))

	cbool := C.render_page_from_document(c_path, C.int(pageNum), c_output)
	return bool(cbool)
}

// Get the text content of the page.
func (page *Page) Text() string {
	g_text := C.poppler_page_get_text(page.page)
	if g_text == nil {
		return ""
	}
	defer C.g_free(C.gpointer(g_text))

	// skip all arrows
	skipToken := []rune{0x25B6, 0x25B8, 0x25B7, 0x25B9, 0x25BA, 0x25BB, 0x25C0, 0x25C2, 0x25C1, 0x25C3, 0x25C4, 0x25C5, 0x25C6, 0x25C7, 0x25C8, 0x25C9, 0x25CA, 0x25CB, 0x25CC, 0x25CD, 0x25CE, 0x25CF, 0x25D0, 0x25D1, 0x25D2, 0x25D3, 0x25D4, 0x25D5, 0x25D6, 0x25D7, 0x25D8, 0x25D9, 0x25DA, 0x25DB, 0x25DC, 0x25DD, 0x25DE, 0x25DF, 0x25E0, 0x25E1, 0x25E2, 0x25E3, 0x25E4, 0x25E5, 0x25E6, 0x25E7, 0x25E8, 0x25E9, 0x25EA, 0x25EB, 0x25EC, 0x25ED, 0x25EE, 0x25EF, 0x25F0, 0x25F1, 0x25F2, 0x25F3, 0x25F4, 0x25F5, 0x25F6, 0x25F7, 0x25F8, 0x25F9, 0x25FA, 0x25FB, 0x25FC, 0x25FD, 0x25FE, 0x25FF, 0x0080}

	text := C.GoString((*C.char)(g_text))
	text = strings.ReplaceAll(text, "\u0089", "")
	for _, token := range skipToken {
		text = strings.ReplaceAll(text, string(token), "")
	}
	return text
}

type Match struct {
	PageNum  int    // Page number
	Filename string // Pointer to filename of the pdf file.
	BaseName string // Filebase name
	Text     string // Line containing the match
	Context  string // Text around the match

	// Hash of the filepath, generate by fnv.New32a hash function.
	ID uint32

	// Relevance score of the match
	Score float32
}

type Matches []Match

func (page *Page) Search(ctx context.Context, pattern string) Matches {
	select {
	case <-ctx.Done():
		return nil
	default:
		return GetPageMatches(page.Text(), pattern, page.doc.Path, page.PageNum)
	}
}

func GetPageMatches(pageText string, pattern string, pdfPath string, pageNum int) Matches {
	lines := strings.Split(pageText, "\n")
	matches := make([]Match, 0, len(lines))
	pattern = strings.ToLower(pattern)

	queryWords := strings.Fields(pattern)

	// Search for the pattern in each line
	for i, line := range lines {
		lowerline := strings.ToLower(line)

		alreadyExists := func(line string) bool {
			return slices.ContainsFunc(matches, func(m Match) bool {
				return m.Text == line
			})
		}

		lineScore := 0.0
		hasMatch := false

		for _, qw := range queryWords {
			if strings.Contains(lowerline, qw) {
				lineScore += 1.0
				hasMatch = true
			}
		}

		if hasMatch && !alreadyExists(line) {
			matches = append(matches, Match{
				ID:       GetPathHash(pdfPath),
				PageNum:  pageNum,
				Filename: pdfPath,
				BaseName: filepath.Base(pdfPath),
				Text:     line,
				Context:  GetLineContext(i, &lines),
				Score:    float32(lineScore),
			})
		}

	}
	return slices.Clip(matches)
}

func GetLineContext(lineno int, lines *[]string) string {
	size := len(*lines)
	contextSize := 5
	var start, end int

	if lineno < contextSize {
		start = 0
		end = min((lineno+contextSize)-1, size-1)
	} else {
		start = lineno - contextSize
		end = min((lineno+contextSize)-1, size-1)
	}
	text := strings.Join((*lines)[start:end], " ")

	// trim start until the first space Capital letter
	for i := 0; i < len(text); i++ {
		if text[i] == ' ' {
			continue
		}

		if text[i] >= 'A' && text[i] <= 'Z' {
			text = text[i:]
			break
		}
	}
	return text
}

var mu sync.Mutex
var hashesCache = make(map[uint32]string)

// Generate a unique hash for path.
func GetPathHash(path string) uint32 {
	// Create an FNV-1 hash
	h := fnv.New32()
	h.Write([]byte(path))
	hashInt := h.Sum32()

	mu.Lock()
	defer mu.Unlock()

	hashesCache[hashInt] = path
	return hashInt
}

func GetPathFromHash(hash uint32) (string, bool) {
	mu.Lock()
	defer mu.Unlock()

	value, ok := hashesCache[hash]
	return value, ok
}

type PathCache struct {
	Hashes map[uint32]string // Hashes to file paths
	Paths  map[string]uint32 // File paths to hashes
}

// SerializeCaches writes the caches to the given file.
// Uses gob encoding.
func SerializeCaches(out string, files []string) error {
	// Clear the caches
	hashesCache = make(map[uint32]string)

	// Populate the caches
	for _, file := range files {
		GetPathHash(file)
	}

	cache := PathCache{
		Hashes: hashesCache,
	}

	w, err := os.Create(out)
	if err != nil {
		return err
	}
	defer w.Close()

	enc := gob.NewEncoder(w)
	err = enc.Encode(cache)
	if err != nil {
		return fmt.Errorf("error encoding cache: %v", err)
	}
	return nil
}

// DeserializeCaches reads the cache from the given file and populates the caches.
// Uses gob decoding.
func DeserializeCaches(in string) error {
	r, err := os.Open(in)
	if err != nil {
		return err
	}

	dec := gob.NewDecoder(r)
	var cache PathCache

	err = dec.Decode(&cache)
	if err != nil {
		return fmt.Errorf("error decoding cache: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	hashesCache = cache.Hashes
	return nil
}

// Search searches for the given pattern in all pages of the PDF document.
// It sends the matches to the provided channel.
// It uses a semaphore to limit the number of concurrent searches.
// The function returns when the context is canceled.
// @param ctx: The context used to cancel the search operation.
// @param pattern: The search pattern.
// @param matches: The channel to send the matches to.
// @param maxConcurrency: The maximum number of concurrent searches.
func (pdf *Document) Search(ctx context.Context, pattern string,
	matches chan<- Match, maxConcurrency int) {
	semaphore := make(chan struct{}, maxConcurrency)
	defer close(semaphore)

	g, ctx := errgroup.WithContext(ctx)
	for page := range pdf.NumPages {
		page := page

		// Acquire a slot from the semaphore
		semaphore <- struct{}{}

		g.Go(func() error {
			defer func() {
				// Release the slot back to the semaphore
				<-semaphore
			}()

			page := pdf.GetPage(page)
			defer page.Close()

			for _, match := range page.Search(ctx, pattern) {
				select {
				case matches <- match:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}

	// Close the channel when all goroutines are done
	err := g.Wait()
	if err != nil {
		if err != context.Canceled {
			fmt.Println("Error searching PDF:", err)
		}
	}
	close(matches)
}
