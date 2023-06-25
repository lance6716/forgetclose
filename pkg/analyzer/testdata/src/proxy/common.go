package proxy

import (
	"importee"
)

func NewCloser() (importee.Closer, error) {
	return importee.NewCloser(), nil
}
