package pdf_test

import (
	"testing"

	"github.com/abiiranathan/pdfsearch/pdf"
)

func TestMultiDocument(t *testing.T) {
	tc := []struct {
		path string
	}{
		{path: "/home/nabiizy/Downloads/BNF-2020.pdf"},
		{path: "/home/nabiizy/Downloads/SLE.pdf"},
	}

	for _, c := range tc {
		t.Run(c.path, func(t *testing.T) {
			md, err := pdf.OpenDocuments(c.path)
			if err != nil {
				t.Fatal(err)
			}
			defer md.Close()

			// Get the number of pages in the document
			if len(md.Data) != 1 {
				t.Fatalf("expected 1 document, got %d", len(md.Data))
			}

			multdoc := md.Data[0]

			if multdoc.Path != c.path {
				t.Fatalf("expected %s, got %s", c.path, multdoc.Path)
			}
		})
	}
}
