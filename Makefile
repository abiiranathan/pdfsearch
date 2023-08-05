CC=gcc

all:
	${CC} -O2 -o pdfsearch main.c `pkg-config --cflags --libs  glib-2.0 gio-2.0 cairo poppler-glib` -pthread 

clean:
	rm -f ./pdfsearch *.pdf images/*.png
