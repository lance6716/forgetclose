package interfacetest

import "importee"

func ifErr() {
	c := importee.NewCloser() // want ".*not closed!"
	c = importee.NewCloser()
	if c != nil {
		c.Close()
	}
	c = importee.NewCloser() // want ".*not closed!"
	if c != nil {
		c.Foo()
	}
}
