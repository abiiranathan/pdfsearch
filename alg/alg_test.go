package alg

import (
	"testing"

	"github.com/jdkato/prose/v2"
)

func TestMatchQuery(t *testing.T) {
	type args struct {
		pageText string
		query    string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "MatchQuery",
			args: args{
				pageText: "This is a test",
				query:    "This is a test",
			},
			want: true,
		},
		{
			name: "MatchQuery",
			args: args{
				pageText: "This is a test",
				query:    "does not match",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchQuery(tt.args.pageText, tt.args.query); got != tt.want {
				t.Errorf("MatchQuery() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkMatchQuery(b *testing.B) {
	for i := 0; i < b.N; i++ {
		MatchQuery("This is a test", "This is a test")
	}
}

func BenchmarkCosineSimilarity(b *testing.B) {
	tfidf1 := map[string]float64{
		"this": 1.0,
		"is":   1.0,
		"a":    1.0,
		"test": 1.0,
	}
	tfidf2 := map[string]float64{
		"this": 1.0,
		"is":   1.0,
		"a":    1.0,
		"test": 1.0,
	}
	for i := 0; i < b.N; i++ {
		CalculateCosineSimilarity(tfidf1, tfidf2)
	}
}

// bench CalculateTFIDF
func BenchmarkCalculateTFIDF(b *testing.B) {
	text := "This is a test"
	doc, _ := prose.NewDocument(text)

	for i := 0; i < b.N; i++ {
		CalculateTFIDF(doc)
	}
}
