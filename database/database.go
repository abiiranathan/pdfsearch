package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/abiiranathan/pdfsearch/pdf"
	"github.com/mattn/go-sqlite3"
)

var db *sql.DB

// Connect to sqlite3 database.
func Connect(dbname string) *sql.DB {
	var err error
	db, err = sql.Open("sqlite3", dbname)
	if err != nil {
		log.Fatalf("unable to connect to database: %v\n", err)
	}

	// ping the database to ensure we are connected.
	err = db.Ping()
	if err != nil {
		log.Fatalf("unable to ping database: %v\n", err)
	}

	// Enable foreign key constraints and WAL mode.
	_, err = db.Exec(`PRAGMA foreign_keys = ON ; PRAGMA journal_mode = WAL`)
	if err != nil {
		log.Fatalf("unable to set pragma: %v\n", err)
	}

	return db
}

func CreateTables() error {
	// create table for filenames
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS files(
		id INTEGER NOT NULL PRIMARY KEY,
		name TEXT NOT NULL,
		path TEXT NOT NULL UNIQUE
	)
	`)

	if err != nil {
		return err
	}

	// Create virtual table with the sqlite3 FTS5 extension to use for our searches.
	// Virtual tables do not support primary key.
	// See official documentation for more information: https://www.sqlite.org/fts5.html
	// tokenize='porter unicode61 remove_diacritics 2' is used to tokenize the text.
	// This is a custom tokenizer that uses the porter stemmer, unicode61 tokenization, and removes diacritics.
	_, err = db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS pages USING fts5(
			file_id UNINDEXED,
			page_num UNINDEXED,
			text,
			tokenize='porter unicode61 remove_diacritics 2'
		)
	`)
	if err != nil {
		return err
	}
	return err
}

func GetFiles(ctx context.Context) ([]File, error) {
	query := `SELECT id, name, path FROM files ORDER BY name`

	files := []File{}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var file File
		err := rows.Scan(&file.ID, &file.Name, &file.Path)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return files, nil
}

func GetFile(ctx context.Context, fileId int) (file File, err error) {
	query := `SELECT id, name, path FROM files WHERE id=$1 LIMIT 1`

	row := db.QueryRowContext(ctx, query, fileId)
	err = row.Scan(&file.ID, &file.Name, &file.Path)
	return
}

func InsertFiles(ctx context.Context, files []File) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	numFiles := len(files)
	if numFiles == 0 {
		return nil
	}

	// Split files into batches
	// This is done to avoid hitting the SQLITE_MAX_VARIABLE_NUMBER limit of 999
	batchSize := 500
	for i := 0; i < numFiles; i += batchSize {
		end := i + batchSize
		if end > numFiles {
			end = numFiles
		}

		batch := files[i:end] // end is exclusive, no out of bounds error
		placeholder, args := fileValueTuple(&batch)
		query := fmt.Sprintf("INSERT INTO files (id, name, path) VALUES %s", placeholder)
		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return err
		}
		defer stmt.Close()

		_, err = stmt.ExecContext(ctx, args...)
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	log.Printf("Inserted %d files into the database\n", numFiles)
	return nil
}

// Insert files one by one, ignoring any conflicts.
func InsertOneByOne(ctx context.Context, files []File) error {
	query := `INSERT INTO files (id, name, path) VALUES ($1, $2, $3) ON CONFLICT(path) DO NOTHING`
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, file := range files {
		_, err := tx.ExecContext(ctx, query, pdf.GetPathHash(file.Path), filepath.Base(file.Path), file.Path)
		if err != nil {
			if sqliteErr, ok := err.(sqlite3.Error); ok {
				if sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
					log.Printf("file %s already exists in the database\n", file.Path)
				} else {
					return fmt.Errorf("error inserting file %s: %w", file.Path, err)
				}
			} else {
				return fmt.Errorf("error inserting file %s: %w", file.Path, err)
			}
		}
	}
	return tx.Commit()

}

// Perform a full-text search on the pages table.
func Search(ctx context.Context, pattern string, books ...int) ([]SearchResult, error) {
	var query = `SELECT DISTINCT file_id, page_num, snippet(pages, 2, '<b>', '</b>','...', 16) title,
		snippet(pages, 2, '<b>', '</b>','...', 60) text,
		files.name AS base_name
        FROM pages JOIN files ON pages.file_id = files.id 
		WHERE pages MATCH $1 order by rank limit 200`

	if len(books) > 0 {
		query = `SELECT DISTINCT file_id, page_num, snippet(pages, 2, '<b>', '</b>','...', 16) title,
		snippet(pages, 2, '<b>', '</b>','...', 60) text,
		files.name AS base_name
        FROM pages 
		JOIN files ON pages.file_id = files.id
		WHERE pages MATCH $1 AND file_id IN ($2) order by rank limit 200`
	}

	results := []SearchResult{}
	args := []interface{}{pattern}
	if len(books) > 0 {
		args = append(args, books)
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var result SearchResult
		err := rows.Scan(&result.FileID, &result.PageNum, &result.Title, &result.Text, &result.BaseName)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return results, nil
}

// Insert a page into the pages table.
func InsertPage(ctx context.Context, page Page) error {
	query := `INSERT INTO pages (file_id, page_num, text) VALUES($1, $2, $3) 
			  ON CONFLICT(file_id, page_num) IGNORE`
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, query, page.FileID, page.PageNum, page.Text)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Insert multiple pages into the pages table at once.
func InsertPagesOneByOne(ctx context.Context, pages []Page) error {
	query := `INSERT INTO pages (file_id, page_num, text) VALUES($1, $2, $3)`
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, page := range pages {
		_, err := tx.ExecContext(ctx, query, page.FileID, page.PageNum, page.Text)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func InsertPages(ctx context.Context, pages []Page) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	numPages := len(pages)
	if numPages == 0 {
		return nil
	}

	log.Printf("Storing %d pages into the database. This may take a minute or two!!", numPages)
	// Split pages into batches
	// This is done to avoid hitting the SQLITE_MAX_VARIABLE_NUMBER limit of 999
	batchSize := 500
	for i := 0; i < numPages; i += batchSize {
		end := i + batchSize
		if end > numPages {
			end = numPages
		}

		batch := pages[i:end]
		placeholders, args := pageValueTuple(&batch)
		query := fmt.Sprintf("INSERT INTO pages (file_id, page_num, text) VALUES %s", placeholders)
		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return err
		}
		defer stmt.Close()

		_, err = stmt.ExecContext(ctx, args...)
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	log.Printf("Inserted %d pages into the database\n", numPages)
	return nil
}

func pageValueTuple(pages *[]Page) (string, []interface{}) {
	query := ""
	var args []interface{}
	for _, page := range *pages {
		// Use placeholders for values
		query += "(?, ?, ?),"
		args = append(args, page.FileID, page.PageNum, page.Text)
	}
	// Remove trailing comma
	query = strings.TrimSuffix(query, ",")

	return query, args
}

func fileValueTuple(files *[]File) (string, []interface{}) {
	query := ""
	var args []interface{}
	for _, file := range *files {
		// Use placeholders for values
		query += "(?, ?, ?),"
		args = append(args, file.ID, file.Name, file.Path)
	}
	// Remove trailing comma
	query = strings.TrimSuffix(query, ",")
	return query, args
}
