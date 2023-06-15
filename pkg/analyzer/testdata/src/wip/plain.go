package wip

import "database/sql"

func readAndClose(r *sql.Rows) {
	for r.Next() {
		var name string
		r.Scan(&name)
	}
	r.Close()
}

func readNotClose(r *sql.Rows) {
	for r.Next() {
		var name string
		r.Scan(&name)
	}
}

func functest() {
	rows, _ := db.Query("SELECT name FROM users")
	readAndClose(rows)

	rows, _ = db.Query("SELECT name FROM users") // want ".*not closed!"
	readNotClose(rows)
}
