//go:build !sqlite_vec
// +build !sqlite_vec

package conv

import (
	_ "github.com/tursodatabase/go-libsql"
)

// sqliteDriverName is the SQL driver name for libsql builds.
const sqliteDriverName = "libsql"
