package testcase

import "database/sql"

func closer(f func() error) {
	f()
}

func deferCloser() {
	rows, _ := db.Query("SELECT name FROM users")
	defer closer(rows.Close)
}

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

func funcTest() {
	rows, _ := db.Query("SELECT name FROM users")
	readAndClose(rows)

	rows, _ = db.Query("SELECT name FROM users") // want ".*not closed!"
	readNotClose(rows)
}
