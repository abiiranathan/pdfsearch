package main

import (
	"embed"
	"log"
	"os"

	"github.com/abiiranathan/pdfsearch/cli"
	"github.com/abiiranathan/pdfsearch/database"
	"github.com/abiiranathan/pdfsearch/pdf"
	"github.com/abiiranathan/pdfsearch/server"
)

const (
	// Temporary storage for generated images
	pagesDir = "pages"

	// Path to the database
	dbPath = "pdfsearch.db"
)

//go:embed all:templates
var viewsFs embed.FS

//go:embed static
var staticFs embed.FS

// Default configuration for the CLI
var config = &cli.DefaultConfig

func startServer() {
	server.Run(config, pagesDir, viewsFs, staticFs)
}

func main() {
	log.SetPrefix("[pdfsearch]: ")
	log.SetFlags(log.Lshortfile)

	// Set the locale to the system's default
	pdf.SetLocale()

	// Parse the command line arguments
	ctx := cli.DefineFlags(config, startServer)
	subcmd, err := ctx.Parse(os.Args)
	if err != nil {
		log.Fatalln(err)
	}

	// If the subcommand is nil, print the usage and exit
	if subcmd == nil {
		ctx.PrintUsage(os.Stdout)
		os.Exit(1)
	}

	// Connect to the database or fail if we cannot connect.
	database.Connect(dbPath)
	if err := database.CreateTables(); err != nil {
		log.Fatalf("unable to create tables: %v\n", err)
	}

	// Run the subcommand
	subcmd.Handler()
}
