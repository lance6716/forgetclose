package wip

import (
	"context"
	"database/sql"
)

var (
	ctx context.Context
	db  *sql.DB
)

func return2() (*sql.Rows, error) {
	rows, err := db.Query("SELECT name FROM users")
	if err != nil {
		return db.Query("SELECT name FROM users")
	}
	rows.Next()
	return rows, nil
}
