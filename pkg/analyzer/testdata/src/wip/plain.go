package wip

import (
	"context"
	"database/sql"
)

var (
	ctx context.Context
	db  *sql.DB
)

func deferClosure() {
	rows, _ := db.Query("SELECT name FROM users")
	defer func() {
		rows.Close()
	}()
}
