package wip

import (
	"context"
	"database/sql"
)

var (
	ctx context.Context
	db  *sql.DB
)

func namedReturn() (_ string, err error) {
	rows, err := db.Query("SELECT name FROM users")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	return "", nil
}
