package wip

import (
	"context"
	"database/sql"
)

var (
	ctx context.Context
	db  *sql.DB
)

func deferClosureArgNotClose() {
	rows, _ := db.Query("SELECT name FROM users")
	defer func() {
		if rows != nil {
			rows.Close()
		}
	}()
}
