//go:build cgo

package database

import (
	"database/sql"

	sqlite3 "github.com/mattn/go-sqlite3"
)

func init() {
	// mattn/go-sqlite3 registers as "sqlite3"; register "sqlite" as an alias so
	// all sql.Open("sqlite", ...) calls work regardless of build mode.
	// Use recover because multiple packages may attempt this registration.
	defer func() { recover() }()
	sql.Register("sqlite", &sqlite3.SQLiteDriver{})
}
