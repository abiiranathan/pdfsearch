#ifndef B7F67327_B077_4053_BDA2_92437D428A76
#define B7F67327_B077_4053_BDA2_92437D428A76

#include <cairo/cairo-pdf.h>
#include <cairo/cairo.h>
#include <locale.h>
#include <poppler/glib/poppler.h>
#include <stdbool.h>
#include <stdio.h>

#ifdef _WIN32
#include <windows.h>
#else
#include <pthread.h>
#endif

// Open a PDF document and return a PopplerDocument object.
// The number of pages in the document is stored in the num_pages parameter.
PopplerDocument* open_document(const char* filename, int* num_pages);

// Render a pdf of a Poppler Page with cairo.
bool poppler_page_to_pdf(PopplerPage* page, const char* output_pdf);

// Render a page of a PDF document to a PNG image.
// The image is scaled to fit within the specified width and height.
// This function is thread-safe and uses a mutex to prevent concurrent access to the cairo library.
void render_page_to_image(PopplerPage* page, int width, int height, const char* output_file);

// Render a page of a PDF document to a PNG image.
// This function is thread-safe and uses a mutex to prevent concurrent access to the cairo library.
// Call this to avoid the overhead of opening and closing the PDF document for each page across
// the cgo boundary.
bool render_page_from_document(const char* pdf_path, int page_num, const char* output_png);

// Takes in the page num, pdf path, outpdf path and renders the page to a pdf
// Calls poppler_page_to_pdf to render the page but avoids multiple cgo calls
// to open the document and get the page.
// Returns true if the page was rendered successfully, false otherwise.
bool render_page_to_pdf(const char* pdf_path, int page_num, const char* output_pdf);

// Read the whole PDF file in parallel extracting the text from each page.
// The text is stored in the text parameter, which must be freed by the caller.
// The number of pages in the document is stored in the num_pages parameter.
char** read_pdf_text(const char* filename, int* num_pages, int num_threads);

// free_pdf_text frees the memory allocated for the text extracted from a PDF file.
void free_pdf_text(char** text, int num_pages);

#endif /* B7F67327_B077_4053_BDA2_92437D428A76 */
