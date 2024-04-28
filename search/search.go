package search

import (
	"bytes"
	"cmp"
	"context"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/abiiranathan/pdfsearch/pdf"
	"github.com/bbalet/stopwords"
	"github.com/jdkato/prose/v2"
	"golang.org/x/sync/errgroup"
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

func SearchDirectory(ctx context.Context, dir string, pattern string, maxConcurrency int) (pdf.Matches, error) {
	files, err := WalkDir(dir, []string{".pdf"})
	if err != nil {
		return nil, err
	}

	results := make([]pdf.Match, 0, len(files)*10)

	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, maxConcurrency)
	defer close(semaphore)

	wg.Add(len(files))
	for _, file := range files {
		// Acquire a slot from the semaphore
		semaphore <- struct{}{}

		go func(file string) {
			defer func() {
				// Release the slot back to the semaphore
				<-semaphore
				wg.Done()
			}()
			searchFile(ctx, file, pattern, &results, maxConcurrency)
		}(file)
	}

	wg.Wait()
	return results, nil
}

func searchFile(ctx context.Context, file string, pattern string, collector *[]pdf.Match, maxConcurrency int) {
	doc := pdf.Open(file)
	if doc == nil {
		panic("Error opening PDF")
	}
	defer doc.Close()

	fileChan := make(chan pdf.Match, 10)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		doc.Search(ctx, pattern, fileChan, maxConcurrency)
	}()

	for match := range fileChan {
		*collector = append(*collector, match)
	}

	// Wait for the search to complete to avoid goroutine leaks
	wg.Wait()
}

type PageMeta struct {
	PageNum  int    // Page number
	Filename string // Pointer to filename of the pdf file.
	PageText string // Line containing the match
}

func getFileMetadata(file string, collector *[]PageMeta, maxConcurrency int) error {
	doc := pdf.Open(file)
	if doc == nil {
		return fmt.Errorf("unable to open document: %v", file)
	}
	defer doc.Close()

	semaphore := make(chan struct{}, maxConcurrency)
	defer close(semaphore)

	g := errgroup.Group{}
	for page := range doc.NumPages {
		page := page

		// Acquire a slot from the semaphore
		semaphore <- struct{}{}

		g.Go(func() error {
			defer func() {
				<-semaphore
			}()

			p := doc.GetPage(page)
			defer p.Close()

			*collector = append(*collector, PageMeta{
				PageNum:  page,
				Filename: doc.Path,
				PageText: p.Text(),
			})
			return nil
		})

	}
	return g.Wait()
}

func SearchFile(ctx context.Context, file string, pattern string, maxConcurrency int) (pdf.Matches, error) {
	var results []pdf.Match
	searchFile(ctx, file, pattern, &results, maxConcurrency)
	return results, nil
}

const pathCacheFilename = "paths.bin"

type mapValue struct {
	index    IndexKey
	pageText string
}

func SearchFromIndex(pattern string, searchIndex *SearchIndex, books ...uint32) (pdf.Matches, error) {
	matches := make(pdf.Matches, 0, 500)
	numWorkers := 4

	jobs := make(chan mapValue, len(*searchIndex))
	results := make(chan pdf.Match, len(*searchIndex))

	// convert pattern to lowercase
	pattern = strings.ToLower(stopwords.CleanString(pattern, "en", false))
	originalPattern := pattern

	// create a pattern document
	patternDoc, err := prose.NewDocument(pattern)
	if err != nil {
		return nil, fmt.Errorf("unable to create pattern document: %v", err)
	}

	// Tokenize the pattern
	patternTokens := patternDoc.Tokens()

	// Identify the keywords(Nouns and Verbs) in the pattern
	/*
		NN: Noun, singular or mass
		VB: Verb, base form
		NNS: Noun, plural
		VBZ: Verb, 3rd person singular present
		JJ: Adjective
	*/
	keywords := make([]string, 0, len(patternTokens))
	nouns := make([]string, 0, len(patternTokens))

	for _, token := range patternTokens {
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
		pattern = strings.Join(keywords, " ")
	}

	var wg sync.WaitGroup
	wg.Add(numWorkers + 1) // +1 for the results channel closer

	threshold := min(len(originalPattern), 10)

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

					hasExactMatch := strings.Contains(strings.ToLower(line), originalPattern)
					lvd := stopwords.LevenshteinDistance([]byte(strings.ToLower(line)), []byte(pattern), "en", false)
					if hasExactMatch || lvd < threshold {
						if hasExactMatch {
							lvd = 0.0 // Lower the score for exact matches
						}

						snippet := pdf.GetLineContext(i, &lines)
						results <- pdf.Match{
							ID:       pdf.GetPathHash(kv.index.Filename),
							Filename: kv.index.Filename,
							BaseName: filepath.Base(kv.index.Filename),
							PageNum:  kv.index.Page,
							Text:     line,
							Context:  snippet,
							Score:    float32(lvd),
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
					jobs <- mapValue{
						index:    pdfNamePage,
						pageText: pageText,
					}
				}
			} else {
				jobs <- mapValue{
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

	// Remove duplicates
	deduped := make(map[string]pdf.Match)
	for _, match := range matches {
		key := fmt.Sprintf("%d:%s:%d", match.PageNum, match.Text, match.ID)
		if _, ok := deduped[key]; !ok {
			deduped[key] = match
		}
	}

	matches = make(pdf.Matches, 0, len(deduped))
	for _, match := range deduped {
		matches = append(matches, match)
	}

	// Sort matches by score least LV distance
	slices.SortStableFunc(matches, func(a, b pdf.Match) int {
		return cmp.Compare(a.Score, b.Score)
	})

	// Clip and return matches
	return slices.Clip(matches), nil
}

type IndexKey struct {
	Filename string
	Page     int
}

// Index contains a given pdf and page with each page text.
// [{Name, 10}: "This is page text"]
type SearchIndex map[IndexKey]string

func Serialize(directory string, outfile string, maxConcurrency int) error {
	files, err := WalkDir(directory, []string{".pdf"})
	if err != nil {
		return fmt.Errorf("unable to load files at %s: %v", directory, err)
	}

	log.Printf("Found %d files in %s\n", len(files), directory)

	results := make([]PageMeta, 0, len(files)*2)
	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, maxConcurrency)
	defer close(semaphore)

	count := len(files)
	wg.Add(count)

	for i, file := range files {
		i := i + 1 // Not required in go1.22 but I don't want lint errors.
		// Acquire a slot from the semaphore
		semaphore <- struct{}{}

		go func(file string) {
			defer func() {
				// Release the slot back to the semaphore
				<-semaphore
				wg.Done()
			}()

			err := getFileMetadata(file, &results, maxConcurrency)
			if err != nil {
				log.Println(err)
			}
			fmt.Printf("(%d/%d) Processed: %s\n", i, count, file)
		}(file)
	}

	wg.Wait()

	// Build the index
	index := BuildIndex(&results)

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

	log.Println("Index written to ", outfile)

	// write paths to cache
	log.Println("Writing paths to cache")
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

func BuildIndex(data *[]PageMeta) *SearchIndex {
	m := make(SearchIndex, len(*data))

	for _, match := range *data {
		key := IndexKey{Filename: match.Filename, Page: match.PageNum}
		if _, ok := m[key]; !ok {
			m[key] = match.PageText
		}
	}
	return &m
}
