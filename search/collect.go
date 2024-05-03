package search

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/abiiranathan/pdfsearch/pdf"
)

// Mutex to lock collector slice to avoid race condition during append.
var collectorMU sync.Mutex

// Read pdf file and append its read pages into the collector.
func CollectPages(file string, collector *[]Page) error {
	doc := pdf.Open(file)
	if doc == nil {
		return fmt.Errorf("error opening document")
	}
	defer doc.Close()

	numWorkers := runtime.NumCPU()
	jobs := make(chan int, numWorkers)
	results := make(chan Page, numWorkers)

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	// Start worker goroutines to perform the search in parallel.
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()

			for page := range jobs {
				func(page int) {
					p := doc.GetPage(page)
					defer p.Close()
					text := p.Text()

					results <- Page{
						PageNum:  page,
						Filename: doc.Path,
						Text:     text,
					}
				}(page)
			}
		}()
	}

	// Send jobs to workers
	go func() {
		for page := 0; page < doc.NumPages; page++ {
			jobs <- page
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for result := range results {
		collectorMU.Lock()
		*collector = append(*collector, result)
		collectorMU.Unlock()
	}
	return nil
}
