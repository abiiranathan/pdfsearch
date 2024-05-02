#include "pdf.h"

#ifdef _WIN32
static CRITICAL_SECTION cairo_mutex;
#else
static pthread_mutex_t cairo_mutex = PTHREAD_MUTEX_INITIALIZER;
#endif

// Define cross-platform mutex functions in separate functions
static void init_cairo_mutex() {
#ifdef _WIN32
    InitializeCriticalSection(&cairo_mutex);
#else
    pthread_mutex_init(&cairo_mutex, NULL);
#endif
}

// Destroy the mutex when it is no longer needed.
void destroy_cairo_mutex() {
#ifdef _WIN32
    DeleteCriticalSection(&cairo_mutex);
#else
    pthread_mutex_destroy(&cairo_mutex);
#endif
}

// lock the mutex
static void lock_cairo_mutex() {
#ifdef _WIN32
    EnterCriticalSection(&cairo_mutex);
#else
    pthread_mutex_lock(&cairo_mutex);
#endif
}

// unlock the mutex
static void unlock_cairo_mutex() {
#ifdef _WIN32
    LeaveCriticalSection(&cairo_mutex);
#else
    pthread_mutex_unlock(&cairo_mutex);
#endif
}

// Open a PDF document and return the number of pages
PopplerDocument* open_document(const char* filename, int* num_pages) {
    GFile* file = g_file_new_for_path(filename);
    if (file == NULL) {
        return NULL;
    }

    GError* error = NULL;
    GBytes* bytes = g_file_load_bytes(file, NULL, NULL, &error);
    g_object_unref(file);

    if (error != NULL) {
        g_print("%s\n", error->message);
        g_clear_error(&error);
        return NULL;
    }

    PopplerDocument* doc = poppler_document_new_from_bytes(bytes, NULL, &error);
    if (error) {
        g_print("%s\n", error->message);
        g_clear_error(&error);
        g_bytes_unref(bytes);
        return NULL;
    }

    *num_pages = poppler_document_get_n_pages(doc);
    g_bytes_unref(bytes);
    return doc;
}

void render_page_to_image(PopplerPage* page, int width, int height, const char* output_file) {
    // Set the desired resolution (300 DPI)
    double resolution = 300.0;
    int pixel_width = (int)(width * resolution / 72.0);
    int pixel_height = (int)(height * resolution / 72.0);

    // Lock the mutex before creating the Cairo surface
    lock_cairo_mutex();

    // Create the Cairo surface with the specified resolution
    cairo_surface_t* surface =
        cairo_image_surface_create(CAIRO_FORMAT_ARGB32, pixel_width, pixel_height);
    if (surface == NULL) {
        unlock_cairo_mutex();
        puts("Unable to create cairo surface");
        return;
    }

    cairo_t* cr = cairo_create(surface);
    if (cr == NULL) {
        unlock_cairo_mutex();
        puts("Error: could not create cairo context");
        return;
    }

    // Set the background color to white (or any desired color)
    cairo_set_source_rgb(cr, 1.0, 1.0, 1.0);  // White background
    cairo_paint(cr);                          // Fill the surface with the background color

    // Disable anti-aliasing for text rendering to avoid blurriness.
    cairo_set_antialias(cr, CAIRO_ANTIALIAS_NONE);

    // Calculate the scaling factors to maintain the original aspect ratio
    double scale_x = (double)pixel_width / width;
    double scale_y = (double)pixel_height / height;
    cairo_scale(cr, scale_x, scale_y);

    poppler_page_render(page, cr);

    // Unlock the mutex after rendering the page
    unlock_cairo_mutex();

    cairo_status_t status = cairo_surface_write_to_png(surface, output_file);
    if (status != CAIRO_STATUS_SUCCESS) {
        puts("Error: could not write to png file");
        return;
    }
    cairo_surface_destroy(surface);
    cairo_destroy(cr);
}

// Render a single page from a document. Avoids multiple cgo calls
bool render_page_from_document(const char* pdf_path, int page_num, const char* output_png) {
    int num_pages = 0;
    PopplerDocument* doc = open_document(pdf_path, &num_pages);
    if (doc == NULL) {
        puts("Error opening document");
        return false;
    }

    if (page_num < 0 || page_num >= num_pages) {
        puts("Page number is out of range of this document");
        g_object_unref(doc);
        return false;
    }

    PopplerPage* page = poppler_document_get_page(doc, page_num);
    if (page == NULL) {
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
bool poppler_page_to_pdf(PopplerPage* page, const char* output_pdf) {
    cairo_surface_t* surface;
    cairo_t* cr;

    double width, height;
    poppler_page_get_size(page, &width, &height);

    double resolution = 300.0;
    int pixel_width = (int)(width * resolution / 72.0);
    int pixel_height = (int)(height * resolution / 72.0);

    lock_cairo_mutex();

    surface = cairo_pdf_surface_create(output_pdf, pixel_width, pixel_height);
    if (cairo_surface_status(surface) != CAIRO_STATUS_SUCCESS) {
        puts("Error creating PDF surface");
        unlock_cairo_mutex();
        return false;
    }

    cr = cairo_create(surface);
    if (cairo_status(cr) != CAIRO_STATUS_SUCCESS) {
        puts("Error creating cairo context");
        unlock_cairo_mutex();
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

    unlock_cairo_mutex();
    return true;
}

// Takes in the page num, pdf path, outpdf path and renders the page to a pdf
// Calls poppler_page_to_pdf to render the page but avoids multiple cgo calls
// to open the document and get the page.
// Returns true if the page was rendered successfully, false otherwise.
bool render_page_to_pdf(const char* pdf_path, int page_num, const char* output_pdf) {
    int num_pages = 0;
    PopplerDocument* doc = open_document(pdf_path, &num_pages);
    if (doc == NULL) {
        puts("Error opening document");
        return false;
    }

    if (page_num < 0 || page_num >= num_pages) {
        puts("Page number is out of range of this document");
        g_object_unref(doc);
        return false;
    }

    PopplerPage* page = poppler_document_get_page(doc, page_num);
    if (page == NULL) {
        printf("PopplerPage for page %d is NULL\n", page_num);
        g_object_unref(doc);
        return false;
    }

    bool status = poppler_page_to_pdf(page, output_pdf);
    g_object_unref(doc);
    g_object_unref(page);
    return status;
}

static void process_page(PopplerPage* page, gpointer user_data) {
    GPtrArray* text_array = (GPtrArray*)user_data;
    char* text = poppler_page_get_text(page);
    if (text != NULL) {
        g_ptr_array_add(text_array, g_strdup(text));
        g_free(text);
    }
}

// Read the whole PDF file in parallel extracting the text from each page.
// The text is stored in the text parameter, which must be freed by the caller.
// The number of pages in the document is stored in the num_pages parameter.
char** read_pdf_text(const char* filename, int* num_pages, int num_threads) {
    PopplerDocument* doc = open_document(filename, num_pages);
    if (doc == NULL) {
        puts("Error opening document");
        return NULL;
    }

    *num_pages = poppler_document_get_n_pages(doc);

    GPtrArray* text_array = g_ptr_array_new();

    // process pages with thread pool provided by glib
    GThreadPool* pool = g_thread_pool_new((GFunc)process_page, text_array, num_threads, TRUE, NULL);
    for (int i = 0; i < *num_pages; i++) {
        PopplerPage* page = poppler_document_get_page(doc, i);
        g_thread_pool_push(pool, page, NULL);
    }

    g_thread_pool_free(pool, FALSE, TRUE);

    // Allocate memory for the array of strings
    char** text = (char**)malloc(text_array->len * sizeof(char*));
    for (int i = 0; i < text_array->len; i++) {
        text[i] = g_strdup(g_ptr_array_index(text_array, i));
    }

    g_ptr_array_free(text_array, TRUE);

    g_object_unref(doc);
    return text;
}

void free_pdf_text(char** text, int num_pages) {
    if (text == NULL) {
        return;
    }

    for (int i = 0; i < num_pages; i++) {
        g_free(text[i]);
    }
    g_free(text);
}

struct OpenDocumentArgs {
    const char* filename;
    PopplerDocument* doc;
    int* num_pages;
};

void* async_open_document(struct OpenDocumentArgs* args) {
    PopplerDocument* doc = open_document(args->filename, args->num_pages);
    return doc;
}

// Open multiple documents simultaneously
bool open_documents(MDocument* md, const char** filenames, size_t num_files) {
    GThread* threads[num_files];
    memset(md, 0, num_files * sizeof(MDocument));

    for (size_t i = 0; i < num_files; i++) {
        struct OpenDocumentArgs* args = g_new(struct OpenDocumentArgs, 1);
        args->filename = filenames[i];
        args->num_pages = &md[i].num_pages;
        threads[i] = g_thread_new(NULL, (GThreadFunc)async_open_document, (gpointer)args);
    }

    for (size_t i = 0; i < num_files; i++) {
        PopplerDocument* doc = g_thread_join(threads[i]);
        if (doc == NULL) {
            // free the documents that have been opened so far
            for (size_t j = 0; j < i; j++) {
                g_object_unref(md[j].document);
            }
            return false;
        }
        md[i].document = doc;
    }
    return true;
}

void free_documents(MDocument** md, size_t num_files, bool free_array) {
    if (md == NULL)
        return;

    for (size_t i = 0; i < num_files; i++) {
        g_object_unref(md[i]->document);
    }

    if (free_array) {
        free(md);
        md = NULL;
    }
}