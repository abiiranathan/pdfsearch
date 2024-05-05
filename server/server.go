package server

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/abiiranathan/pdfsearch/cli"
	"github.com/abiiranathan/pdfsearch/routes"
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
	routes.SetupRoutes(mux, staticFS, pagesDir, tmpl)

	// Clean up temporary files every 2 minutes.
	go cleanUpTemporaryFiles(pagesDir)

	go func() {
		defer GracefulShutdown(server)
	}()

	log.Printf("Listening on http://0.0.0.0:%d\n", config.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("unable to start server: %v\n", err)
	}
}

// Gracefully shuts down the server. The default timeout is 10 seconds
// To wait for pending connections.
func GracefulShutdown(server *http.Server, timeout ...time.Duration) {
	var t time.Duration
	if len(timeout) > 0 {
		t = timeout[0]
	} else {
		t = time.Second * 10
	}

	ctx, cancel := context.WithTimeout(context.Background(), t)
	defer cancel()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	cancel()

	log.Println("Shutting down server")
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("unable to shutdown server: %v\n", err)
	}

	log.Println("Server shutdown")
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
