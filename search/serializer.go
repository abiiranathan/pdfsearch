package search

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/abiiranathan/pdfsearch/pdf"
)

// Serialize reads all pdfs at directory, processes them in parallel and stores
// the generated index in a binary outfile.
func Serialize(directory string, outfile string, nworkers ...int) error {
	workers := 2
	if len(nworkers) > 0 {
		workers = nworkers[0]
	}

	files, err := WalkDir(directory, []string{".pdf"})
	if err != nil {
		return fmt.Errorf("unable to load files at %s: %v", directory, err)
	}

	numFiles := len(files)
	log.Printf("Found %d files in %s\n", numFiles, directory)
	log.Println("Processing pdfs in", workers, "goroutines")

	results := []Page{}

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

	// Build the index
	log.Println("Building the search index....")
	index := BuildIndex(&results)

	log.Println("Encoding data with GOB encoding...")
	buf := new(bytes.Buffer)
	gobEncoder := gob.NewEncoder(buf)
	err = gobEncoder.Encode(index)
	if err != nil {
		return err
	}

	err = os.WriteFile(outfile, buf.Bytes(), os.ModePerm)
	if err != nil {
		return fmt.Errorf("unable to write index to %s: %v", outfile, err)
	}
	log.Println("SearchIndex written to", outfile)

	// write paths to cache
	err = pdf.SerializeCaches(pathCacheFilename, files)
	if err != nil {
		return fmt.Errorf("unable to write paths to cache: %v", err)
	}
	return err
}

// Deserialize reads the generated index from a file
// and returns a pointer to the SearchIndex.
func Deserialize(index string) (*SearchIndex, error) {
	b, err := os.ReadFile(index)
	if err != nil {
		return nil, err
	}

	searchIndex := make(SearchIndex)
	err = gob.NewDecoder(bytes.NewReader(b)).Decode(&searchIndex)
	if err != nil {
		return nil, err
	}

	// Load paths from cache
	err = pdf.DeserializeCaches(pathCacheFilename)
	if err != nil {
		return nil, fmt.Errorf("unable to load paths from cache: %v", err)
	}
	return &searchIndex, nil
}
