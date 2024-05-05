package search

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"github.com/abiiranathan/pdfsearch/database"
	"github.com/abiiranathan/pdfsearch/pdf"
)

// Used to carry arguments to the collector.
type fileJob struct {
	index int
	name  string
}

// Serialize reads all pdfs at directory, processes them in parallel and stores
// the generated index in a binary outfile.
func Serialize(directory string, once bool, workers int) error {
	files, err := WalkDir(directory, []string{".pdf"})
	if err != nil {
		return fmt.Errorf("unable to load files at %s: %v", directory, err)
	}

	numFiles := len(files)
	log.Printf("Found %d files in %s\n", numFiles, directory)
	log.Println("Processing pdfs in", workers, "goroutines")

	results := []database.Page{}

	var wg sync.WaitGroup
	wg.Add(workers)

	jobs := make(chan fileJob, numFiles/workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()

			for job := range jobs {
				fmt.Printf("[worker-%d] (%d/%d) Processing: %s\n", i, job.index, numFiles, job.name)
				err := CollectPages(job.name, &results)
				if err != nil {
					log.Println("unable to process", job.name)
				}
			}
		}()
	}

	go func() {
		for i, file := range files {
			jobs <- fileJob{index: i, name: file}
		}
		close(jobs)
	}()

	wg.Wait()

	// Create a slice of database.File to store the file information.
	dbFiles := make([]database.File, numFiles)
	for i, file := range files {
		dbFiles[i] = database.File{
			ID:   int(pdf.GetPathHash(file)),
			Name: filepath.Base(file),
			Path: file,
		}
	}

	log.Println("Storing file information into the database")
	if once {
		// Store the file information into the database.
		err := database.InsertFiles(context.Background(), dbFiles)
		if err != nil {
			return fmt.Errorf("unable to store files: %v", err)
		}
	} else {
		// Store the file information into the database one by one.
		err := database.InsertOneByOne(context.Background(), dbFiles)
		if err != nil {
			return fmt.Errorf("unable to store files: %v", err)
		}
	}

	// Store the generated index of results into the database.
	if once {
		return database.InsertPages(context.Background(), results)
	} else {
		return database.InsertPagesOneByOne(context.Background(), results)
	}
}
