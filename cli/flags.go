package cli

import (
	"log"
	"os"

	"github.com/abiiranathan/goflag"
	"github.com/abiiranathan/pdfsearch/search"
)

func DefineFlags(config *Config, runserver func()) *goflag.Context {
	// Create flag context.
	ctx := goflag.NewContext()

	// build_index subcommand
	buildCmd := ctx.AddSubCommand("build_index", "Build a file index for a specified folder", serializeHandler(config))
	buildCmd.AddFlag(goflag.FlagDirPath, "directory", "d", &config.Directory, "The directory to index", true)
	buildCmd.AddFlag(goflag.FlagBool, "once", "o", &config.Once,
		"Bulk file upload(faster but errors on duplicates). Otherwise, use the slow, one-by-one way(ignores duplicates)", false)
	buildCmd.AddFlag(goflag.FlagInt, "workers", "w", &config.NumWorkers,
		"Number of workers to use when processing pdfs", false)

	// Server subcommand
	srv := ctx.AddSubCommand("serve", "Start an Http server for search", runserver)
	srv.AddFlag(goflag.FlagInt, "port", "p", &config.Port, "The port to run the server on", false)

	return ctx
}

func serializeHandler(config *Config) func() {
	return func() {
		err := search.Serialize(config.Directory, config.Once, config.NumWorkers)
		if err != nil {
			log.Fatalf("unable to serialize files: %v\n", err)
		}
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
