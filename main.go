package main

import (
	"embed"
	"log"
	"os"

	"github.com/abiiranathan/pdfsearch/cli"
	"github.com/abiiranathan/pdfsearch/pdf"
	"github.com/abiiranathan/pdfsearch/server"
)

// Temporary storage for generated images
const pagesDir = "pages"

//go:embed all:templates
var viewsFs embed.FS

//go:embed static
var staticFs embed.FS

// Default configuration for the CLI
var config = &cli.DefaultConfig

func startServer() {
	cli.ValidateIndex(config.Index)
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

	// Run the subcommand
	subcmd.Handler()
}
