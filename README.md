# pdfsearch

This is an experimental PDF search engine. The project is written go and uses the C API for poppler-glib and cairo to extract text from PDF files.

## Motivation
I am a medical doctor and I have a lot of PDF files with notes, books, and articles. I wanted to have a simple search engine to search for keywords in these files. I also wanted to learn more about the go programming language's CGO capabilities.

## Installation
You need to have the poppler-glib and cairo libraries installed on your system. On Ubuntu, you can install them with the following command:

```bash
sudo apt-get install libpoppler-glib-dev libcairo2-dev pkg-config
```

If that does not work, you can try the following command:
```bash
sudo apt-get install build-essential cmake pkg-config\
 libpoppler-glib-dev glib2.0 glib2.0-dev libfontconfig1-dev \
 libfreetype6-dev libjpeg-dev libpng-dev libtiff-dev \
 libopenjp2-7-dev libcurl4-gnutls-dev libgtest-dev libboost-dev
```


On Arch Linux, you can install them with the following command:

```bash
sudo pacman -S poppler-glib cairo pkg-config
```

If that does not work, you can try the following command:
```bash
sudo pacman -S base-devel cmake pkg-config poppler-glib glib2 fontconfig freetype2 libjpeg-turbo libpng libtiff libcurl-gnutls gtest boost
```


Then you can install the pdfsearch tool with the following command:

```bash
git clone https://github.com/abiiranathan/pdfsearch.git
cd pdfsearch
go install # or go build
```

On Windows, download the unofficial poppler-glib and cairo libraries from the following links:

1. [poppler-glib - Github Release 24](https://github.com/oschwartz10612/poppler-windows/releases/tag/v24.02.0-0)

Folder Structure:
```txt
├── Library
│   ├── bin
│   │   ├── cairo.dll
│   │   ├── charset.dll
│   │   ├── deflate.dll
│   │   ├── expat.dll
│   │   ├── fontconfig-1.dll
│   │   ├── freetype.dll
│   │   ├── iconv.dll
│   │   ├── jpeg8.dll
│   │   ├── lcms2.dll
│   │   ├── Lerc.dll
│   │   ├── libcrypto-3-x64.dll
│   │   ├── libcurl.dll
│   │   ├── libexpat.dll
│   │   ├── liblzma.dll
│   │   ├── libpng16.dll
│   │   ├── libssh2.dll
│   │   ├── libtiff.dll
│   │   ├── libzstd.dll
│   │   ├── openjp2.dll
│   │   ├── pdfattach.exe
│   │   ├── pdfdetach.exe
│   │   ├── pdffonts.exe
│   │   ├── pdfimages.exe
│   │   ├── pdfinfo.exe
│   │   ├── pdfseparate.exe
│   │   ├── pdftocairo.exe
│   │   ├── pdftohtml.exe
│   │   ├── pdftoppm.exe
│   │   ├── pdftops.exe
│   │   ├── pdftotext.exe
│   │   ├── pdfunite.exe
│   │   ├── pixman-1-0.dll
│   │   ├── poppler-cpp.dll
│   │   ├── poppler.dll
│   │   ├── poppler-glib.dll
│   │   ├── tiff.dll
│   │   ├── zlib.dll
│   │   ├── zstd.dll
│   │   └── zstd.exe
│   ├── include
│   │   └── poppler
│   ├── lib
│   │   ├── pkgconfig
│   │   ├── poppler-cpp.lib
│   │   ├── poppler-glib.lib
│   │   └── poppler.lib
│   └── share
│       └── man
└── share
    └── poppler
        ├── cidToUnicode
        ├── CMakeLists.txt
        ├── cMap
        ├── COPYING
        ├── COPYING.adobe
        ├── COPYING.gpl2
        ├── Makefile
        ├── nameToUnicode
        ├── poppler-data.pc
        ├── poppler-data.pc.in
        ├── README
        └── unicodeMap


```

## USAGE

1. Step 1: Index the PDF files
```bash
./pdfsearch build_index -d /path/to/directory/of/pdf/files
```
The default index file is `~/index.bin` in the current directory. You can specify a different index file with the `-i` flag.

This command will create an binary index file with the text extracted from the PDF files in the specified directory.

2. Run the web server
```bash
./pdfsearch runserver -p 8080 -i /path/to/index.bin
```

This command will start a web server on port 8080. You can specify a different port with the `-p` flag. You can specify a different index file with the `-i` flag.

3. Open the web browser and go to `http://localhost:8080` to search for keywords in the PDF files.