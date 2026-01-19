//go:build !sqlite_vec
// +build !sqlite_vec

package parser

import (
	"database/sql"
	"strings"

	_ "github.com/tursodatabase/go-libsql"
)

const sqliteDriverName = "libsql"

func openCursorDB(sourcePath string) (*sql.DB, error) {
	dsn := sourcePath
	if !strings.HasPrefix(dsn, "file:") {
		dsn = "file:" + dsn + "?mode=ro"
	}
	return sql.Open(sqliteDriverName, dsn)
}
