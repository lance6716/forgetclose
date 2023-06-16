package wip

import (
	"context"
	"database/sql"
)

var (
	ctx context.Context
	db  *sql.DB
)

func checkNilBeforeClose(r *sql.Rows) {
	if r == nil {
		return
	}
	r.Close()
}

func deferClosureArgNotClose() {
	rows, _ := db.Query("SELECT name FROM users")
	checkNilBeforeClose(rows)
}
