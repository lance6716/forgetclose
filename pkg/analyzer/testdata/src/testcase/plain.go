package testcase

import "database/sql"

func plain() {
	rows, _ := db.Query("SELECT name FROM users") // want ".*not closed!"
	rows.Next()
}

func plain2() {
	rows, _ := db.Query("SELECT name FROM users")
	rows.Close()
}

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

func overwrite() {
	rows, _ := db.Query("SELECT name FROM users") // want ".*not closed!"
	rows, _ = db.Query("SELECT name FROM users")
	rows.Close()
}

func ifTest() {
	rows, _ := db.Query("SELECT name FROM users") // want ".*not closed!"
	var i int
	rows.Scan(&i)
	if i%2 == 0 {
		rows.Close()
	}
}

func ifErr() {
	rows, err := db.Query("SELECT name FROM users")
	if err != nil {
		return
	}
	rows.Close()

	rows, err = db.Query("SELECT name FROM users")
	if err == nil {
		rows.Close()
	}
}

func underscore() {
	_, err := db.Query("SELECT name FROM users") // want ".*not closed!"
	if err != nil {
		return
	}
}

func forTest() {
	rows, err := db.Query("SELECT name FROM users")
	if err != nil {
		return
	}

	for rows.Next() {
		var name string
		rows.Scan(&name)
	}
	rows.Close()

	rows, _ = db.Query("SELECT name FROM users") // want ".*not closed!"

	for rows.Next() {
		var name string
		rows.Scan(&name)
		if name == "" {
			return
		}
	}
	rows.Close()
}

func deferTest() {
	rows, err := db.Query("SELECT name FROM users")
	if err != nil {
		return
	}

	defer rows.Close()

	rows, err = db.Query("SELECT name FROM users") // want ".*not closed!"
	if err != nil {
		return
	}
}

func deferClosure() {
	rows, _ := db.Query("SELECT name FROM users")
	defer func() {
		rows.Close()
	}()
}

func deferClosureNotClose() {
	rows, _ := db.Query("SELECT name FROM users") // want ".*not closed!"
	defer func() {
		rows.Next()
	}()
}

func deferNotClose() {
	rows, _ := db.Query("SELECT name FROM users") // want ".*not closed!"
	defer rows.Next()
}

func deferClosureArg() {
	rows, err := db.Query("SELECT name FROM users")
	if err != nil {
		return
	}
	defer func(rs *sql.Rows) {
		_ = rs.Close()
	}(rows)
}

func deferClosureArgNotClose() {
	rows, err := db.Query("SELECT name FROM users") // want ".*not closed!"
	if err != nil {
		return
	}
	defer func(rs *sql.Rows) {
		_ = rs.Next()
	}(rows)
}

func deferClosureIfCheck() {
	rows, _ := db.Query("SELECT name FROM users")
	defer func() {
		if rows != nil {
			rows.Close()
		}
	}()
}

func return2() (*sql.Rows, error) {
	rows, err := db.Query("SELECT name FROM users")
	if err != nil {
		return db.Query("SELECT name FROM users")
	}
	rows.Next()
	return rows, nil
}

func namedReturn() (_ string, err error) {
	rows, err := db.Query("SELECT name FROM users")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	return "", nil
}

func shadow() {
	var rows *sql.Rows
	rows, err := db.Query("SELECT name FROM users")
	if err != nil {
		return
	}
	defer rows.Close()
}

func notUse() {
	db.Query("SELECT name FROM users") // want ".*not closed!"
}
