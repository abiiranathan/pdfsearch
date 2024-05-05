package cli

// Config holds the configuration for the CLI.
type Config struct {
	// the directory to index
	Directory string

	// Bulk file upload(faster but errors on duplicates).
	// Otherwise, use the slow, one-by-one way(ignores duplicates)
	Once bool

	// Number of workers to use when processing pdfs. Default is 2.
	NumWorkers int

	// server port. default is 8080
	Port int
}

var DefaultConfig = Config{
	Port:       8080,
	Once:       true,
	NumWorkers: 2,
}
