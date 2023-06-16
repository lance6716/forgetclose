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

type holder struct {
	rows *sql.Rows
}

func asField() {
	rows, _ := db.Query("SELECT name FROM users")
	h := holder{rows: rows}
	h.rows.Close()
}

func checkNilBeforeClose(r *sql.Rows) {
	if r == nil {
		return
	}
	r.Close()
}

func nilTest() {
	rows, _ := db.Query("SELECT name FROM users")
	checkNilBeforeClose(rows)
}
