package search

import (
	"cmp"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/abiiranathan/pdfsearch/pdf"
	"github.com/bbalet/stopwords"
	"github.com/jdkato/prose/v2"
)

const pathCacheFilename = "paths.bin"

// Key in search index map.
type IndexKey struct {
	Filename string // Absolute path to the PDF on disk.
	Page     int    // The page number (zero-indexed)
}

// SearchIndex is map of IndexKey(pdfName, page) to the page text.
type SearchIndex map[IndexKey]string

// used by the collector while processing multiple pages in parallel.
type pageJob struct {
	index    IndexKey
	pageText string
}

// Used to carry arguments to the collector.
type fileJob struct {
	index int
	name  string
}

type Page struct {
	PageNum  int    // Page number as in index(zero-indexed) in the PopplerDocument
	Filename string // Pointer to filename of the pdf file.
	Text     string // Page text for the PopplerPage.
}

// Returns true if the string str contains any element in arr.
func stringContainsAny(str string, arr []string) bool {
	for _, item := range arr {
		if strings.Contains(str, item) {
			return true
		}
	}
	return false
}

// BuildIndex builds a search index from Pages of pdf text.
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

var indexPattern = regexp.MustCompile(`(.+)[,\s]\s*(\d+)(.*)?`)

func isPageIndex(pageText string) bool {
	return indexPattern.MatchString(pageText)

}

// EntryPoint to the search engine.
// Searches the index for a given query and returns the matches.
// If any books are provided in arguments, only matches from these books are returned.
func Search(query string, searchIndex *SearchIndex, books ...uint32) (pdf.Matches, error) {
	const (
		NownSingular        = "NN"
		Verb                = "VB"
		NownPlural          = "NNS"
		VerbSingularPresent = "VBZ"
		Adjective           = "JJ"

		numWorkers int = 10
		maxMatches int = 200
	)

	matches := make(pdf.Matches, 0, maxMatches) // Initialize slice for matches
	threshold := float32(min(len(query), 10))   // Threshold for the Levenistein distance

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

	keywords := make([]string, 0, len(queryTokens))
	nouns := make([]string, 0, len(queryTokens))

	for _, token := range queryTokens {
		if token.Tag == NownSingular ||
			token.Tag == Verb ||
			token.Tag == NownPlural ||
			token.Tag == VerbSingularPresent ||
			token.Tag == Adjective {
			keywords = append(keywords, token.Text)
		}

		if token.Tag == NownSingular || token.Tag == NownPlural {
			nouns = append(nouns, token.Text)
		}
	}

	if len(keywords) > 0 {
		query = strings.Join(keywords, " ")
	}

	var wg sync.WaitGroup
	wg.Add(numWorkers + 1) // +1 for the results channel closer

	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()

			for kv := range jobs {
				lines := strings.Split(kv.pageText, "\n")

				// Am not interested in the book index
				if isPageIndex(kv.pageText) {
					continue
				}

				for i, line := range lines {
					// If the line does not contains the keyword, skip it
					if !stringContainsAny(line, keywords) {
						continue
					}

					// If no nown is found in the line, skip it
					if !stringContainsAny(line, nouns) {
						continue
					}

					// if the line contains an exact match to query
					hasExactMatch := strings.Contains(strings.ToLower(line), origQuery)

					// Compute the string Levenshtein Distance
					levisteinDistance := float32(stopwords.LevenshteinDistance(
						[]byte(strings.ToLower(line)), []byte(query), "en", false))

					if hasExactMatch || levisteinDistance < threshold {
						if hasExactMatch {
							// The exact match is given a higher score (smaller distance)
							levisteinDistance = float32(len(origQuery)+len(line)) / 100
						}

						// generate a snippet of text around the current line.
						snippet := pdf.GetLineContext(i, &lines)

						// send match on the results channel.
						results <- pdf.Match{
							ID:       pdf.GetPathHash(kv.index.Filename),
							Filename: kv.index.Filename,
							BaseName: filepath.Base(kv.index.Filename),
							PageNum:  kv.index.Page,
							Text:     line,
							Context:  snippet,
							Score:    levisteinDistance,
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
		wg.Done()
	}()

	// Collect results
	wg.Wait()
	close(results)

	seen := make(map[string]struct{}, maxMatches)
	for match := range results {
		if _, exists := seen[match.Text]; !exists {
			seen[match.Text] = struct{}{}
			matches = append(matches, match)
		}
	}

	// Sort matches by score least LV distance
	slices.SortStableFunc(matches, func(a, b pdf.Match) int {
		return cmp.Compare(a.Score, b.Score)
	})

	// Clip the matches to a maximum of 200
	if len(matches) > maxMatches {
		matches = matches[:maxMatches]
	}
	return matches, nil
}
