package cli

// Config holds the configuration for the CLI.
type Config struct {
	// Max files or folders processes at a time.
	// Large values will increase CPU and memory usage.
	// Default is 10.
	MaxConcurrency int

	// the name of the file to store the folder index.
	Index string

	// the directory to index
	Directory string

	// the file to search directory
	Filename string

	// Search pattern / regex
	Pattern string

	// server port. default is 8080
	Port int
}

var DefaultConfig = Config{
	MaxConcurrency: 10,
	Port:           8080,
}
