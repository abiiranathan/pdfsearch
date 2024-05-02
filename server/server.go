package server

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/abiiranathan/pdfsearch/cli"
	"github.com/abiiranathan/pdfsearch/routes"
	"github.com/abiiranathan/pdfsearch/search"
)

func Run(config *cli.Config, pagesDir string, viewsFs embed.FS, staticFS embed.FS) {
	// Create the pages directory if it does not exist
	// We use this to store the generated images from pdfs.
	err := os.MkdirAll(pagesDir, os.ModePerm)
	if err != nil {
		log.Fatalf("unable to create directory: %s: %v\n", pagesDir, err)
	}

	// Parse templates.
	tmpl, err := template.ParseFS(viewsFs, "templates/*.html")
	if err != nil {
		// we panic because we cannot proceed without the templates
		panic(fmt.Errorf("unable to parse templates: %v", err))
	}

	// build the index
	searchIndex, err := search.Deserialize(config.Index)
	if err != nil {
		panic(fmt.Errorf("unable to deserialize index: %s", config.Index))
	}

	// Create a new serveMux
	mux := http.NewServeMux()

	// Create a new http server to customize the timeouts.
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", config.Port),
		Handler:           routes.Logger(os.Stdout)(mux),
		ReadTimeout:       time.Second * 10,
		WriteTimeout:      time.Second * 10,
		ReadHeaderTimeout: time.Second * 5,
	}

	// Connect the routes.
	routes.SetupRoutes(mux, staticFS, pagesDir, tmpl, searchIndex)

	// Clean up temporary files every 2 minutes.
	go cleanUpTemporaryFiles(pagesDir)

	defer GracefulShutdown(server)

	log.Printf("Listening on http://0.0.0.0:%d\n", config.Port)

	// Start the server
	err = server.ListenAndServe()
	if err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server terminated with error: %v\n", err)
		}
	}
}

func cleanUpTemporaryFiles(dir string) {
	ticker := time.NewTicker(time.Minute)
	quit := make(chan struct{})

	proc := func() {
		files, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".pdf") {
				os.Remove(filepath.Join(dir, file.Name()))
			}
		}
		log.Println("Cleaning up generated images")
	}

	for {
		select {
		case <-ticker.C:
			proc()
		case <-quit:
			ticker.Stop()
		}
	}

}

// Gracefully shuts down the server. The default timeout is 10 seconds
// To wait for pending connections.
func GracefulShutdown(server *http.Server, timeout ...time.Duration) {
	var t time.Duration
	if len(timeout) > 0 {
		t = timeout[0]
	} else {
		t = 10 * time.Second
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	log.Println("waiting on os.Interrupt")

	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), t)
	defer cancel()

	log.Println("Shutting down the server")
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalln(err)
	}
	log.Println("shutting down gracefully")
}
