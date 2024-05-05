package database

// Store files with their base name and path to the file system.
type File struct {
	ID   int
	Name string
	Path string
}

// A page in a file. Related by FileID.
type Page struct {
	FileID  int    //  ID of the file this page belongs to.
	PageNum int    // 0-indexed page number.
	Text    string // Full text of the page.
}

// A snippet of text from a page. Related by FileID and PageNum.
type SearchResult struct {
	FileID   int    // ID of the file this page belongs to.
	PageNum  int    // Page number
	Title    string // Snippet representing the title of the match
	Text     string // Snippet of text from the page
	BaseName string // Filebase name of the file
}
