package search

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func WalkDir(dir string, extensions []string) ([]string, error) {
	var files []string

	// Search for PDF files in the directory
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the directory itself and the parent directory
		if path == dir || path == "." || path == ".." {
			return nil
		}

		// Skip hidden files
		if d.Name()[0] == '.' {
			return filepath.SkipDir
		}

		ext := filepath.Ext(strings.ToLower(path))
		if d.Type().IsRegular() && slices.Contains(extensions, ext) {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return files, nil
}
