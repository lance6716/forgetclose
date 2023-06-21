package importee

type Closer interface {
	Foo()
	Close() error
}

type impl struct{}

func (i *impl) Foo()         {}
func (i *impl) Close() error { return nil }

func NewCloser() Closer {
	return &impl{}
}
