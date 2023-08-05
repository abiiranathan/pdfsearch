# pdfsearch

Fast Multithreaded PDF Search.

## Dependencies

```bash
 sudo apt-get install build-essential cmake pkg-config\
 libpoppler-glib-dev glib2.0 glib2.0-dev libfontconfig1-dev \
 libfreetype6-dev libjpeg-dev libpng-dev libtiff-dev \
 libopenjp2-7-dev libcurl4-gnutls-dev libgtest-dev libboost-dev
```

Compile:

```bash
make
```

Run:

```bash
./pdfsearch -h

Usage: ./pdfsearch [options] filename search_term[Regex Pattern]
Options:
  -h, --help            Display this help message.
  -c, --context         Specify the context size for displaying the surrounding text of matched words (default: 100).
  -t, --threads         Specify the number of threads for multi-threaded search (default: 10).
  -s, --save-images     Save images of pages with matches.
  -p, --path    Directory to save images.

```

Prints the match with context of 100. (ie 50 characters before and after the match).

### Vcode Setup

c_cpp_properties.json

```json
{
  "configurations": [
    {
      "name": "Linux",
      "includePath": [
        "${workspaceFolder}/**",
        "/usr/include/glib-2.0", // glib-object.h, glib.h, gobject
        "/usr/lib/x86_64-linux-gnu/glib-2.0/include", // glibconfig.h
        "/usr/include/cairo/" //cairo.h
      ],
      "defines": [],
      "compilerPath": "/usr/bin/clang",
      "cStandard": "c17",
      "intelliSenseMode": "linux-gcc-x64"
    }
  ],
  "version": 4
}


clangd: settings.json

{
  "clangd.path": "/usr/bin/clangd",
  "clangd.fallbackFlags": [
    "-I${workspaceFolder}/**",
    "-I/usr/include/glib-2.0",
    "-I/usr/lib/x86_64-linux-gnu/glib-2.0/include",
    "-I/usr/include/cairo/",
    "-I/usr/include/poppler/**"
  ],
  "clangd.checkUpdates": false,
  "clangd.serverCompletionRanking": true
}

```
