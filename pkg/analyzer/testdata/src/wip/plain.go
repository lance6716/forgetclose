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
	rows, err := db.Query("SELECT name FROM users") // want ".*not closed!"
	if err != nil {
		return
	}
	defer func(rs *sql.Rows) {
		_ = rs.Next()
	}(rows)
}
