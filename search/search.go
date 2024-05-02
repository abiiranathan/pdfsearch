package search

import (
	"bytes"
	"cmp"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/abiiranathan/pdfsearch/pdf"
	"github.com/bbalet/stopwords"
	"github.com/jdkato/prose/v2"
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

type Page struct {
	PageNum  int    // Page number as in index(zero-indexed) in the PopplerDocument
	Filename string // Pointer to filename of the pdf file.
	Text     string // Page text for the PopplerPage.
}

// Mutex to lock collector slice to avoid race condition during append.
var collectorMU sync.Mutex

func CollectPages(file string, collector *[]Page) error {
	doc := pdf.Open(file)
	if doc == nil {
		return fmt.Errorf("doc is nil")
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

const pathCacheFilename = "paths.bin"

type pageJob struct {
	index    IndexKey
	pageText string
}

type fileJob struct {
	index int
	name  string
}

func Search(query string, searchIndex *SearchIndex, books ...uint32) (pdf.Matches, error) {
	matches := make(pdf.Matches, 0, 100)
	const numWorkers int = 10

	jobs := make(chan pageJob, len(*searchIndex))
	results := make(chan pdf.Match, len(*searchIndex))

	// Store a reference to original Query
	origQuery := query

	// convert pattern to lowercase
	query = strings.ToLower(stopwords.CleanString(query, "en", false))

	// create a query document
	queryDoc, err := prose.NewDocument(query, prose.WithExtraction(false), prose.WithSegmentation(false))
	if err != nil {
		return nil, fmt.Errorf("unable to create pattern document: %v", err)
	}

	// Tokenize the query document
	queryTokens := queryDoc.Tokens()

	// Identify the keywords(Nouns and Verbs) in the pattern
	/*
		NN: Noun, singular or mass
		VB: Verb, base form
		NNS: Noun, plural
		VBZ: Verb, 3rd person singular present
		JJ: Adjective
	*/
	keywords := make([]string, 0, len(queryTokens))
	nouns := make([]string, 0, len(queryTokens))

	for _, token := range queryTokens {
		if token.Tag == "NN" ||
			token.Tag == "VB" ||
			token.Tag == "NNS" ||
			token.Tag == "VBZ" ||
			token.Tag == "JJ" {
			keywords = append(keywords, token.Text)
		}

		if token.Tag == "NN" || token.Tag == "NNS" {
			nouns = append(nouns, token.Text)
		}
	}

	if len(keywords) > 0 {
		query = strings.Join(keywords, " ")
	}

	var wg sync.WaitGroup
	wg.Add(numWorkers + 1) // +1 for the results channel closer

	threshold := min(len(origQuery), 10)

	// Start worker goroutines to perform the search in parallel.
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()

			for kv := range jobs {
				lines := strings.Split(kv.pageText, "\n")
				for i, line := range lines {
					// If the line does not contains the keyword, skip it
					continueSearch := false
					for _, keyword := range keywords {
						if strings.Contains(line, keyword) {
							continueSearch = true
							break
						}
					}

					if !continueSearch {
						continue
					}

					// If no nown is found in the line, skip it
					hasNoun := false
					for _, noun := range nouns {
						if strings.Contains(line, noun) {
							hasNoun = true
							break
						}
					}

					if !hasNoun {
						continue
					}

					hasExactMatch := strings.Contains(strings.ToLower(line), origQuery)
					lvd := stopwords.LevenshteinDistance([]byte(strings.ToLower(line)), []byte(query), "en", false)
					lvdFloat := float32(lvd)

					if hasExactMatch || lvd < threshold {
						if hasExactMatch {
							// The exact match is given a higher score (smaller distance)
							lvdFloat = float32(len(origQuery)+len(line)) / 100
						}

						snippet := pdf.GetLineContext(i, &lines)

						// Create a new match
						results <- pdf.Match{
							ID:       pdf.GetPathHash(kv.index.Filename),
							Filename: kv.index.Filename,
							BaseName: filepath.Base(kv.index.Filename),
							PageNum:  kv.index.Page,
							Text:     line,
							Context:  snippet,
							Score:    lvdFloat,
						}
					}
				}
			}
		}()
	}

	// Send jobs to workers
	go func() {
		searchSpecificBooks := len(books) > 0
		for pdfNamePage, pageText := range *searchIndex {
			if searchSpecificBooks {
				currentID := pdf.GetPathHash(pdfNamePage.Filename)
				if slices.Contains(books, currentID) {
					jobs <- pageJob{
						index:    pdfNamePage,
						pageText: pageText,
					}
				}
			} else {
				jobs <- pageJob{
					index:    pdfNamePage,
					pageText: pageText,
				}
			}
		}
		close(jobs)

		// Account for the +1 in the wait group
		// otherwise we leak this goroutine
		wg.Done()
	}()

	// Collect results
	wg.Wait()
	close(results)

	for match := range results {
		matches = append(matches, match)
	}

	// Remove duplicates but keep the order
	deduped := make(pdf.Matches, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if _, ok := seen[match.Text]; !ok {
			seen[match.Text] = struct{}{}
			deduped = append(deduped, match)
		}
	}

	// Sort matches by score least LV distance
	slices.SortStableFunc(deduped, func(a, b pdf.Match) int {
		return cmp.Compare(a.Score, b.Score)
	})

	// Clip the matches to a maximum of 200
	const maxMatches = 200
	if len(deduped) > maxMatches {
		deduped = deduped[:maxMatches]
	}
	return deduped, nil
}

type IndexKey struct {
	Filename string
	Page     int
}

// Index contains a given pdf and page with each page text.
// [{Name, 10}: "This is page text"]
type SearchIndex map[IndexKey]string

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

	// Initialize slice to collect all pages of all documents.
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

func BuildIndex(data *[]Page) *SearchIndex {
	m := make(SearchIndex, len(*data))

	for _, match := range *data {
		key := IndexKey{Filename: match.Filename, Page: match.PageNum}
		if _, ok := m[key]; !ok {
			m[key] = match.Text
		}
	}
	return &m
}
