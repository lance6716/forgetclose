package testcase

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

func iftest() {
	rows, _ := db.Query("SELECT name FROM users") // want ".*not closed!"
	var i int
	rows.Scan(&i)
	if i%2 == 0 {
		rows.Close()
	}
}

func iferr() {
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

func fortest() {
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

func defertest() {
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

func deferclosure() {
	rows, _ := db.Query("SELECT name FROM users")
	defer func() {
		rows.Close()
	}()
}
