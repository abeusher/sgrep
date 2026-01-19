//go:build sqlite_vec
// +build sqlite_vec

package parser

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

const sqliteDriverName = "sqlite3"

func openCursorDB(sourcePath string) (*sql.DB, error) {
	return sql.Open(sqliteDriverName, sourcePath)
}
