package wip

import (
	"context"
	"database/sql"
)

var (
	ctx context.Context
	db  *sql.DB
)

func returnTarget() *sql.Rows {
	rows, _ := db.Query("SELECT name FROM users")
	return rows
}

func returnTest() {
	rows := returnTarget()
	rows.Close()
	rows = returnTarget() // want ".*not closed!"
	returnTarget()        // want ".*not closed!"
}
