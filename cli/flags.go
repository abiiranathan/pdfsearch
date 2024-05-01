package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/abiiranathan/goflag"
	"github.com/abiiranathan/pdfsearch/pdf"
	"github.com/abiiranathan/pdfsearch/search"
)

func DefineFlags(config *Config, runserver func()) *goflag.Context {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("os.UserHomeDir() unable failed: %v\n", err)
	}

	// Create the index file if it does not exist.
	config.Index = filepath.Join(home, "index.bin")
	if _, err := os.Stat(config.Index); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			_, err = os.Create(config.Index)
			if err != nil {
				log.Fatalln(err)
			}
		}
	}

	// Flags required by multiple subcomands
	indexFlag := goflag.Flag{
		FlagType:  goflag.FlagFilePath,
		Name:      "index",
		ShortName: "i",
		Value:     &config.Index,
		Usage:     "The path to the generated binary index or where to write it to",
		Required:  false,
		Validator: nil,
	}

	// Create flag context.
	ctx := goflag.NewContext()

	// global flags
	concurrencyValidators := []func(any) (bool, string){goflag.Min(1), goflag.Max(100)}
	ctx.AddFlag(goflag.FlagInt, "concurrency", "c", &config.MaxConcurrency,
		"No of concurrent processes", false, concurrencyValidators...)

	// build_index subcommand
	buildCmd := ctx.AddSubCommand("build_index", "Build a file index for a specified folder", buildHandler(config))
	buildCmd.AddFlag(goflag.FlagDirPath, "directory", "d", &config.Directory, "The directory to index", true)
	buildCmd.AddFlagPtr(&indexFlag)

	// Search subcommand
	searchCmd := ctx.AddSubCommand("search", "Search the generated index and print matches", searchHandler(config))
	searchCmd.AddFlagPtr(&indexFlag)
	searchCmd.AddFlag(goflag.FlagString, "pattern", "p", &config.Pattern, "The search query", true)

	// Server subcommand
	srv := ctx.AddSubCommand("serve", "Start an Http server for search", runserver)
	srv.AddFlag(goflag.FlagInt, "port", "p", &config.Port, "The port to run the server on", false)
	srv.AddFlagPtr(&indexFlag)

	return ctx
}

func searchHandler(config *Config) func() {
	return func() {
		ValidateIndex(config.Index)

		searchIndex, err := search.Deserialize(config.Index)
		if err != nil {
			panic(fmt.Errorf("unable to deserialize: %s", config.Index))
		}

		matches, err := search.Search(config.Pattern, searchIndex)
		if err != nil {
			log.Fatalln(err)
		}
		printMatches(&matches)
	}
}

func buildHandler(config *Config) func() {
	return func() {
		search.Serialize(config.Directory, config.Index, config.MaxConcurrency)
	}
}

func printMatches(matches *pdf.Matches) {
	for _, match := range *matches {
		fmt.Printf("%s Page: %d : %s\n", match.Filename, match.PageNum, match.Text)
	}
}

func ValidateIndex(index string) {
	stat, err := os.Stat(index)
	if err != nil {
		log.Fatalln("The file you specified for the index does not exist")
	}

	if stat.Size() == 0 {
		log.Fatalf("The file you specified for the index is empty. Run the `build_index` command to generate an index\n")
	}
	log.Printf("Using index: %s [%d bytes]\n", index, stat.Size())
}
