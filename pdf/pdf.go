package pdf

/*
#cgo CFLAGS: -I .

#ifdef _WIN32
error "Windows is not supported yet"
#else
#cgo pkg-config: glib-2.0 gio-2.0 cairo poppler-glib
#cgo LDFLAGS: -lglib-2.0 -lgobject-2.0 -lgio-2.0 -lcairo -lpoppler-glib
#endif

#include "pdf.h"

*/
import "C"
import (
	"fmt"
	"hash/fnv"
	"runtime"
	"strings"
	"unicode"
	"unsafe"
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

	// unable to get the page
	if p_page.page == nil {
		return nil
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

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
	}()

	g_text := C.poppler_page_get_text(page.page)
	if g_text == nil {
		return ""
	}
	defer C.g_free(C.gpointer(g_text))
	return cleanText(C.GoString(g_text))
}

func cleanText(text string) string {
	var cleaned strings.Builder

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if isOnlyDotsOrNumbers(line) {
			continue
		}

		for _, char := range line {
			if unicode.IsLetter(char) || unicode.IsNumber(char) || unicode.IsSpace(char) {
				cleaned.WriteRune(char)
			}
		}
		cleaned.WriteString("\n")
	}
	return cleaned.String()
}

func isOnlyDotsOrNumbers(line string) bool {
	line = strings.TrimSpace(line)
	// return true if line contains only dots or numbers
	for _, char := range line {
		if char != '.' && !unicode.IsNumber(char) {
			return false
		}
	}
	return true
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

// Generate a unique hash for path.
func GetPathHash(path string) uint32 {
	// Create an FNV-1 hash
	h := fnv.New32()
	h.Write([]byte(path))
	hashInt := h.Sum32()
	return hashInt
}

type PathCache struct {
	Hashes map[uint32]string // Hashes to file paths
	Paths  map[string]uint32 // File paths to hashes
}
