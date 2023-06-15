package testcase

func closer(f func() error) {
	f()
}

func defercloser() {
	rows, _ := db.Query("SELECT name FROM users")
	defer closer(rows.Close)
}
