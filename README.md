# pdfsearch

This is an experimental PDF search engine. The project is written go and uses the C API for poppler-glib and cairo to extract text from PDF files.

## Motivation
I am a medical doctor and I have a lot of PDF files with notes, books, and articles. I wanted to have a simple search engine to search for keywords in these files. I also wanted to learn more about the go programming language's CGO capabilities.

## Installation

> You can download the latest release for Linux from the following link:
[Github pdfsearch Releases](https://github.com/abiiranathan/pdfsearch/releases)

### Build from source  
Download the required libraries.

You need to have the poppler-glib and cairo libraries installed on your system. 

> On Ubuntu/debain, you can install them with the following command:
```bash
sudo apt-get install libpoppler-glib-dev libcairo2-dev pkg-config
```

If that does not work, you can try the following command to install all the required libraries:
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

> We have not tested the Windows build. If you have any issues, please let us know.

## USAGE

1. Step 1: Index the PDF files
```bash
./pdfsearch build_index -d /path/to/directory/of/pdf/files
```
The default index file is `~/index.bin` in your home directory. You can specify a different index file with the `-i` flag or --index flag. We advise you to use the default index file.

This command will create an binary index file with the text extracted from the PDF files in the specified directory.

1. Run the web server
```bash
./pdfsearch serve -p 8080

# Or specify the index file
./pdfsearch serve -p 8080 -i ~/index.bin
```

This command will start a web server on port 8080. You can specify a different port with the `-p` flag. You can specify a different index file with the `-i` flag.

3. Open the web browser and go to `http://localhost:8080` to search for keywords in the PDF files.