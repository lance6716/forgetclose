package wip

import (
	"context"
	"database/sql"
)

var (
	ctx context.Context
	db  *sql.DB
)

func plain3() {
	r1, _ := db.Query("SELECT name FROM users")
	r2, _ := db.Query("SELECT name FROM users") // want ".*not closed!"
	r1.Close()
	r3, _ := db.Query("SELECT name FROM users")
	r4, _ := db.Query("SELECT name FROM users") // want ".*not closed!"
	r5, _ := db.Query("SELECT name FROM users")
	r5.Close()
	r3.Close()
	r2.Next()
	r4.Next()
}
