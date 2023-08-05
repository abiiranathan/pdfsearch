#include <cairo/cairo.h>
#include <locale.h>
#include <poppler/glib/poppler.h>

#include <pthread.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <wchar.h>

typedef struct search_args {
  PopplerDocument *doc; // Instance of the poppler document
  char *search_term;    // The search term or regex pattern
  char *outputDir;      // Dir Where to store generated PNG Images
  int start_page;       // start page for each thread
  int end_page;         // end page for each thread
  int context_size;     // context-size for each thread
  bool save_images;     // Whether to save images for each page that matches
} search_args;

// Define a mutex to protect the Cairo resources
static pthread_mutex_t cairo_mutex = PTHREAD_MUTEX_INITIALIZER;

void save_page_to_image(PopplerPage *page, int page_num,
                        const char *outputDir) {
  // Get the size of the page
  double width, height;
  poppler_page_get_size(page, &width, &height);

  // Set the desired resolution (300 DPI)
  double resolution = 300.0;
  int pixel_width = (int)(width * resolution / 72.0);
  int pixel_height = (int)(height * resolution / 72.0);

  // Create the Cairo surface with the specified resolution
  cairo_surface_t *surface = cairo_image_surface_create(
      CAIRO_FORMAT_ARGB32, pixel_width, pixel_height);
  if (!surface) {
    wprintf(L"Unable to create cairo surface\n");
    return;
  }

  cairo_t *cr = cairo_create(surface);
  if (!cr) {
    cairo_surface_destroy(surface);
    wprintf(L"Unable to create cairo object\n");
    return;
  }

  // Set the background color to white (or any desired color)
  cairo_set_source_rgb(cr, 1.0, 1.0, 1.0); // White background
  cairo_paint(cr); // Fill the surface with the background color

  // Disable anti-aliasing for text rendering to avoid blurriness.
  cairo_set_antialias(cr, CAIRO_ANTIALIAS_NONE);

  // Calculate the scaling factors to maintain the original aspect ratio
  double scale_x = pixel_width / width;
  double scale_y = pixel_height / height;
  cairo_scale(cr, scale_x, scale_y);

  // We need a mutex to lock cairo resources
  pthread_mutex_lock(&cairo_mutex);
  // Render the PDF page to the Cairo surface
  poppler_page_render(page, cr);

  // Generate the filename with page_num
  char filename[256];
  if (outputDir) {
    // Check if outputDir ends with a trailing slash '/'
    size_t outputDirLen = strlen(outputDir);
    if (outputDir[outputDirLen - 1] == '/') {
      snprintf(filename, sizeof(filename), "%spage_%03d.png", outputDir,
               page_num);
    } else {
      snprintf(filename, sizeof(filename), "%s/page_%03d.png", outputDir,
               page_num);
    }
    mkdir(outputDir, 0755); // Create the directory if it doesn't exist
  } else {
    snprintf(filename, sizeof(filename), "page_%03d.png", page_num);
  }

  // Write the PNG image to file with high quality (compression level 0 for no
  // compression)
  cairo_surface_write_to_png(surface, filename);

  // Convert the Cairo surface to a raylib Image
  //   Image image = {(unsigned char *)cairo_image_surface_get_data(surface),
  //                  (int)width, (int)height, 1, UNCOMPRESSED_R8G8B8};

  // Free resources used for rendering
  cairo_surface_destroy(surface);
  cairo_destroy(cr);
  pthread_mutex_unlock(&cairo_mutex);
}

// Routine to pass to each thread. args is a pointer to search_args.
void *search_thread(void *args) {
  search_args *search_args = (struct search_args *)args;

  // create case-insensitive regex pattern
  GRegex *regex =
      g_regex_new(search_args->search_term, G_REGEX_CASELESS, 0, NULL);

  // iter over each page in range
  for (int page_num = search_args->start_page; page_num < search_args->end_page;
       page_num++) {

    PopplerPage *page = poppler_document_get_page(search_args->doc, page_num);
    if (page == NULL) {
      wprintf(L"Error: could not get page: %d\n", page_num);
      continue;
    }

    // extract text content from the page
    gchar *text = poppler_page_get_text(page);

    // convert unicode char to utf-8
    wchar_t *wtext = calloc(strlen(text) + 1, sizeof(wchar_t));
    mbstowcs(wtext, text, strlen(text));

    char *utf8_text = calloc((strlen(text) * 4) + 1, sizeof(char));
    wcstombs(utf8_text, wtext, strlen(text) * 4);

    // search for the target search term using regex
    GMatchInfo *match_info;
    if (g_regex_match(regex, utf8_text, 0, &match_info)) {
      // Print every matching word and context before and after it.
      while (g_match_info_matches(match_info)) {
        int start_index, end_index, context_size = 100;

        // We can not have negative context size.
        if (search_args->context_size > 0) {
          context_size = search_args->context_size;
        }

        // get the indices of the matches
        g_match_info_fetch_pos(match_info, 0, &start_index, &end_index);
        int match_size = end_index - start_index;

        int start_context =
            (start_index > context_size) ? start_index - context_size : 0;
        int end_context = ((strlen(utf8_text) - end_index) > context_size)
                              ? end_index + context_size
                              : strlen(utf8_text);
        gchar *match_text =
            g_strndup(&utf8_text[start_context], end_context - start_context);

        // Convert to Unicode
        size_t len = strlen(match_text) + 1;
        wchar_t *unicode_text = calloc(len, sizeof(wchar_t));

        // We can't allocate memory for unicode
        if (unicode_text == NULL) {
          wprintf(
              L"Error: Calloc() failed to allocate memory for unicode text\n");
          continue;
        }

        // convert the text to unicode string
        mbstowcs(unicode_text, match_text, len);
        if (unicode_text == NULL) {
          wprintf(L"Error: mbstowcs() failed to convert text to unicode\n");
          continue;
        }

        // Generate png image for this page
        if (search_args->save_images) {
          PopplerPage *page =
              poppler_document_get_page(search_args->doc, page_num);
          if (page) {
            save_page_to_image(page, page_num, search_args->outputDir);
          }
        }

        // Print the context and highlight the match
        wprintf(L"_____________ Page: %d ___________________\n", page_num + 1);
        for (int i = start_context; i < end_context; i++) {
          if (i >= start_index && i < end_index) {
            wprintf(L"\033[47;31m%lc\033[0m", unicode_text[i - start_context]);
          } else {
            wprintf(L"%lc", unicode_text[i - start_context]);
          }
        }
        wprintf(L"\n\n");

        // Free memory
        g_free(match_text);
        free(unicode_text);
        g_match_info_next(match_info, NULL);
      }
    }

    // Clean up
    g_free(text);
    g_match_info_free(match_info);

    free(wtext);
    free(utf8_text);

    // free memory usage by page object
    g_object_unref(page);
  }

  g_regex_unref(regex);
  return NULL;
}

int main(int argc, char *argv[]) {
  // Command line options
  int opt;
  int context_size = 50;    // Default context size
  int num_threads = 10;     // Default number of threads
  bool save_images = false; // Default is not to save images
  char *outputDir = NULL;

  while ((opt = getopt(argc, argv, "hst:c:p:")) != -1) {
    switch (opt) {
    case 'h': // Help option
      wprintf(L"Usage: %s [options] filename search_term[Regex Pattern]\n",
              argv[0]);
      wprintf(L"Options:\n");
      wprintf(L"  -h, --help\t\tDisplay this help message.\n");
      wprintf(L"  -c, --context\t\tSpecify the context size for displaying the "
              "surrounding text of matched words (default: 100).\n");
      wprintf(L"  -t, --threads\t\tSpecify the number of threads for "
              "multi-threaded search (default: 10).\n");
      wprintf(L"  -s, --save-images\tSave images of pages with matches.\n");
      wprintf(L"  -p, --path\tDirectory to save images.\n");
      return EXIT_SUCCESS;
    case 'c': // Context option
      context_size = atoi(optarg);
      break;
    case 't': // Threads option
      num_threads = atoi(optarg);
      break;
    case 's': // Save images option
      save_images = true;
      break;
    case 'p': // Save the path to store images
      outputDir = optarg;
      break;
    default:
      wprintf(L"Usage: %s [options] filename search_term\n", argv[0]);
      return EXIT_FAILURE;
    }
  }

  // Check if the correct number of positional arguments is provided
  if (argc - optind != 2) {
    wprintf(L"Usage: %s [options] filename search_term\n", argv[0]);
    return EXIT_FAILURE;
  }

  char *filepath = argv[optind];
  char *search_term = argv[optind + 1];

  // set the locale to the user's default to handle Unicode characters correctly
  setlocale(LC_ALL, "");

  // Create a GFile object for the provided file path
  GFile *file = g_file_new_for_path(filepath);
  GError *error = NULL;
  // Load the file into GBytes
  GBytes *bytes = g_file_load_bytes(file, NULL, NULL, &error);

  if (bytes == NULL) {
    g_print("Error: %s\n", error->message);
    g_error_free(error);
    g_object_unref(file);
    return EXIT_FAILURE;
  }

  // Create PDF Document object from GBytes
  error = NULL; // Reset error
  PopplerDocument *doc = poppler_document_new_from_bytes(bytes, NULL, &error);
  if (doc == NULL) {
    g_print("Error creating document: %s\n", error->message);
    g_error_free(error);
    g_bytes_unref(bytes);
    g_object_unref(file);
    return EXIT_FAILURE;
  }

  // Get the number of pages in the PDF Document.
  int num_pages = poppler_document_get_n_pages(doc);
  // Adjust the number of threads if needed
  if (num_pages < num_threads) {
    num_threads = num_pages;
  }

  wprintf(L"Searching pdf \"%s\" for the term \"%s\" using %d threads\n",
          filepath, search_term, num_threads);
  wprintf(L"Number of Pages: %d\n", num_pages);

  int pages_per_thread = num_pages / num_threads;

  // Allocate memory for thread arguments and threads
  search_args *thread_args = calloc(num_threads, sizeof(search_args));
  if (!thread_args) {
    wprintf(L"unable to calloc() memory for search args");
    return EXIT_FAILURE;
  }

  pthread_t *threads = calloc(num_threads, sizeof(pthread_t));
  if (!threads) {
    wprintf(L"unable to allocate memory to threads");
    return EXIT_FAILURE;
  }

  // Create and start threads for searching
  for (int i = 0; i < num_threads; i++) {
    thread_args[i].doc = doc;
    thread_args[i].search_term = search_term;
    thread_args[i].start_page = i * pages_per_thread;
    thread_args[i].context_size = context_size;
    thread_args[i].save_images = save_images;
    thread_args[i].outputDir = outputDir;

    // If we are the last page, this is the end page.
    if (i == (num_pages - 1)) {
      thread_args[i].end_page = num_pages;
    } else {
      thread_args[i].end_page = (i + 1) * pages_per_thread;
    }

    // Create and start the threads, passing off the search function with
    // arguments
    int result = pthread_create(&threads[i], NULL, search_thread,
                                (void *)&thread_args[i]);
    if (result != 0) {
      wprintf(L"Error: could not create thread %d\n", i);
      return EXIT_FAILURE;
    }
  }

  // wait for all the threads to complete
  for (int i = 0; i < num_threads; i++) {
    int result = pthread_join(threads[i], NULL);
    if (result != 0) {
      wprintf(L"Error: could not join thread: %d\n", i);
      return EXIT_FAILURE;
    }
  }

  // free memory used by thread arguments and thread objects
  free(thread_args);
  free(threads);

  // Free the file
  g_object_unref(file);

  // Free the bytes
  g_bytes_unref(bytes);

  // Free memory used by the PDF document object.
  g_object_unref(doc);

  return EXIT_SUCCESS;
}
