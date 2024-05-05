# Build the project with full text search support using FTS5 extension
build:
	go build -ldflags='-w -s' --tags "fts5"