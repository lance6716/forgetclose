package interfacetest

import (
	"proxy"
)

func proxyTest() {
	c, _ := proxy.NewCloser() // want ".*not closed!"
	_ = c
}
