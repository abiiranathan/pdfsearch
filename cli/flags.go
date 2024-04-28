package cli

import (
	"context"
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

func printMatches(matches *pdf.Matches) {
	for _, match := range *matches {
		fmt.Printf("%s Page: %d : %s\n", match.Filename, match.PageNum, match.Context)
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

func DefineFlags(config *Config, runserver func()) *goflag.Context {
	// Use the home folder/index.bin as default index.
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("os.UserHomeDir() unable failed: %v\n", err)
	}

	// Create the index file if it does not exist.
	config.Index = filepath.Join(home, "index.bin")
	if _, err := os.Stat(config.Index); err != nil {
		// if the file does not exist, create it.
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
		Usage:     "The path to write the generated binary index",
		Required:  false,
		Validator: nil,
	}

	patternFlag := goflag.Flag{
		FlagType:  goflag.FlagString,
		Name:      "pattern",
		ShortName: "p",
		Value:     &config.Pattern,
		Usage:     "The search term or regex pattern",
		Required:  true,
		Validator: nil,
	}

	// Create flag context.
	ctx := goflag.NewContext()

	// global flags
	ctx.AddFlag(goflag.FlagInt, "concurrency", "c",
		&config.MaxConcurrency,
		"No of concurrent files or folders to be processed at once",
		false, goflag.Min(1), goflag.Max(100))

	// register subcommands
	ctx.AddSubCommand("build_index", "Build a file index for a specified folder", func() {
		search.Serialize(config.Directory, config.Index, config.MaxConcurrency)
	}).AddFlag(goflag.FlagDirPath, "directory", "d", &config.Directory, "The directory to index", true).
		AddFlagPtr(&indexFlag)

	ctx.AddSubCommand("search_file", "Search a single PDF file", func() {
		matches, err := search.SearchFile(context.Background(),
			config.Filename, config.Pattern, config.MaxConcurrency)
		if err != nil {
			log.Fatalln(err)
		}
		printMatches(&matches)
	}).AddFlag(goflag.FlagFilePath, "file", "f", &config.Filename, "The PDF file to search", true).
		AddFlagPtr(&patternFlag)

	ctx.AddSubCommand("search_dir", "Search directory of PDF files recursively", func() {
		matches, err := search.SearchDirectory(context.Background(),
			config.Directory, config.Pattern, config.MaxConcurrency)
		if err != nil {
			log.Fatalln(err)
		}
		printMatches(&matches)
	}).AddFlag(goflag.FlagDirPath, "directory", "d", &config.Directory, "The directory to search", true).
		AddFlagPtr(&patternFlag)

	ctx.AddSubCommand("search", "Search faster from a generated index", func() {
		ValidateIndex(config.Index)

		searchIndex, err := search.Deserialize(config.Index)
		if err != nil {
			panic(fmt.Errorf("unable to deserialize: %s", config.Index))
		}

		matches, err := search.SearchFromIndex(config.Pattern, searchIndex)
		if err != nil {
			log.Fatalln(err)
		}
		printMatches(&matches)

	}).AddFlagPtr(&indexFlag).AddFlagPtr(&patternFlag)

	// Run server
	ctx.AddSubCommand("runserver", "Start an Http server for search", runserver).
		AddFlag(goflag.FlagInt, "port", "p", &config.Port, "The port to run the server on", false).
		AddFlagPtr(&indexFlag)

	return ctx
}
