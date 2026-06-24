//go:build cgo

package browser

import (
	"database/sql"

	sqlite3 "github.com/mattn/go-sqlite3"
)

func init() {
	defer func() { recover() }()
	sql.Register("sqlite", &sqlite3.SQLiteDriver{})
}
