package wip

import (
	"context"
	"database/sql"
)

var (
	ctx context.Context
	db  *sql.DB
)

func notUse() {
	db.Query("SELECT name FROM users") // want ".*not closed!"
}
