//go:build sqlite_vec
// +build sqlite_vec

package conv

import (
	_ "github.com/mattn/go-sqlite3"
)

// sqliteDriverName is the SQL driver name for sqlite_vec builds.
const sqliteDriverName = "sqlite3"
