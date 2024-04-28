package alg

import (
	"math"

	"github.com/jdkato/prose/v2"
)

// CalculateTFIDF calculates the TF-IDF for a given document.
func CalculateTFIDF(doc *prose.Document) map[string]float64 {
	tokens := doc.Tokens()
	tf := make(map[string]float64, len(tokens))
	for _, token := range tokens {
		tf[token.Text]++
	}
	for token := range tf {
		tf[token] /= float64(len(tokens))
	}
	return tf
}

// CalculateCosineSimilarity calculates the cosine similarity between two TF-IDF maps.
func CalculateCosineSimilarity(tfidf1, tfidf2 map[string]float64) float32 {
	dotProduct := 0.0
	magnitude1 := 0.0
	magnitude2 := 0.0

	for term, score1 := range tfidf1 {
		score2, exists := tfidf2[term]
		if exists {
			dotProduct += score1 * score2
		}
		magnitude1 += score1 * score1
	}
	for _, score2 := range tfidf2 {
		magnitude2 += score2 * score2
	}

	magnitude1 = math.Sqrt(magnitude1)
	magnitude2 = math.Sqrt(magnitude2)

	if magnitude1 != 0 && magnitude2 != 0 {
		return float32(dotProduct / (magnitude1 * magnitude2))
	}
	return 0.0
}

func MatchQuery(pageText string, query string) bool {
	doc, _ := prose.NewDocument(pageText)
	tfidf := CalculateTFIDF(doc)
	queryDoc, _ := prose.NewDocument(query)
	queryTFIDF := CalculateTFIDF(queryDoc)
	return CalculateCosineSimilarity(tfidf, queryTFIDF) > 0.5
}
