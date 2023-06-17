package toBeChecked

import (
	"context"
	"database/sql"
)

var (
	ctx context.Context
	db  *sql.DB
)

func shadow() {
	var rows *sql.Rows
	rows, err := db.Query("SELECT name FROM users")
	if err != nil {
		return
	}
	defer rows.Close()
}
