package pdf

/*
#cgo CFLAGS: -I .

#ifdef _WIN32
// TODO: Add windows support
error "Windows is not supported yet"
#else
#cgo pkg-config: glib-2.0 gio-2.0 cairo poppler-glib
#cgo LDFLAGS: -lglib-2.0 -lgobject-2.0 -lgio-2.0 -lcairo -lpoppler-glib
#endif

#include "pdf.h"

*/
import "C"
import (
	"context"
	"encoding/gob"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"runtime"
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

type MultiDocument struct {
	Data []Document

	// The array is storing PopplerDocments, so we keep a reference
	// so we can free it on Close.
	array *C.struct_MDocument
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

func OpenDocuments(pdfPaths ...string) (*MultiDocument, error) {
	if len(pdfPaths) == 0 {
		return nil, fmt.Errorf("no pdf paths provided")
	}

	cPaths := make([]*C.char, len(pdfPaths))
	for i, path := range pdfPaths {
		cPaths[i] = C.CString(path)
	}

	defer func() {
		for i := range cPaths {
			C.free(unsafe.Pointer(cPaths[i]))
		}
	}()

	// create MDocument array
	mdocs := C.calloc(C.size_t(len(pdfPaths)), C.sizeof_MDocument)
	if mdocs == nil {
		return nil, fmt.Errorf("unable to allocate memory")
	}

	ok := C.open_documents((*C.struct_MDocument)(mdocs), &cPaths[0], C.size_t(len(pdfPaths)))
	if !ok {
		return nil, fmt.Errorf("unable to open documents")
	}

	multiData := make([]Document, len(pdfPaths))

	for i := 0; i < len(pdfPaths); i++ {
		// get the pointer to the i-th MDocument with pointer arithmetic, phew!
		mdoc := (*C.MDocument)(unsafe.Pointer(uintptr(mdocs) + uintptr(i)*C.sizeof_MDocument))
		multiData[i] = Document{
			doc:      mdoc.document,
			Path:     pdfPaths[i],
			NumPages: int(mdoc.num_pages),
		}
	}

	multiDoc := &MultiDocument{
		Data:  multiData,
		array: (*C.struct_MDocument)(mdocs),
	}
	return multiDoc, nil
}

func (mdoc *MultiDocument) Close() {
	if mdoc == nil {
		return
	}

	// free Poppler documents in the array
	for i := 0; i < len(mdoc.Data); i++ {
		C.g_object_unref(C.gpointer(mdoc.Data[i].doc))
	}

	// Free the array itself.
	C.free(unsafe.Pointer(mdoc.array))
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
	if page == nil || page.page == nil {
		return ""
	}

	g_text := C.poppler_page_get_text(page.page)
	if g_text == nil {
		return ""
	}
	defer C.g_free(C.gpointer(g_text))

	// skip all arrows
	skipToken := []rune{0x25B6, 0x25B8, 0x25B7, 0x25B9, 0x25BA, 0x25BB, 0x25C0, 0x25C2,
		0x25C1, 0x25C3, 0x25C4, 0x25C5, 0x25C6, 0x25C7, 0x25C8, 0x25C9, 0x25CA, 0x25CB,
		0x25CC, 0x25CD, 0x25CE, 0x25CF, 0x25D0, 0x25D1, 0x25D2, 0x25D3, 0x25D4, 0x25D5,
		0x25D6, 0x25D7, 0x25D8, 0x25D9, 0x25DA, 0x25DB, 0x25DC, 0x25DD, 0x25DE, 0x25DF,
		0x25E0, 0x25E1, 0x25E2, 0x25E3, 0x25E4, 0x25E5, 0x25E6, 0x25E7, 0x25E8, 0x25E9,
		0x25EA, 0x25EB, 0x25EC, 0x25ED, 0x25EE, 0x25EF, 0x25F0, 0x25F1, 0x25F2, 0x25F3,
		0x25F4, 0x25F5, 0x25F6, 0x25F7, 0x25F8, 0x25F9, 0x25FA, 0x25FB, 0x25FC, 0x25FD,
		0x25FE, 0x25FF, 0x0080}

	text := C.GoString((*C.char)(g_text))
	text = strings.ReplaceAll(text, "\u0089", "")
	for _, token := range skipToken {
		text = strings.ReplaceAll(text, string(token), "")
	}
	return C.GoString(g_text)
}

// Read the text content of a PDF file in a single cgo call in parallel.
// This is fast but consumes a lot of memory.
func ReadPDFText(pdfPath string, numThreads ...int) []string {
	var c_path *C.char = C.CString(pdfPath)
	defer C.free(unsafe.Pointer(c_path))

	var numPages C.int
	nthreads := runtime.NumCPU()
	if len(numThreads) > 0 {
		nthreads = numThreads[0]
	}

	pageTextArr := C.read_pdf_text(c_path, &numPages, C.int(nthreads))
	defer C.free_pdf_text(pageTextArr, numPages)

	// Convert the C array to a Go slice
	goPageTextSlice := (*[1 << 30]*C.char)(unsafe.Pointer(pageTextArr))[:numPages:numPages]
	return pagesToStrings(goPageTextSlice, int(numPages))
}

func pagesToStrings(pages []*C.char, numPages int) []string {
	texts := make([]string, numPages)
	for i, page := range pages {
		texts[i] = C.GoString(page)
	}
	return texts
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
	contextSize := 10
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
