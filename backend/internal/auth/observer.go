package auth

type Observer interface {
	LoginFailure()
	RefreshReuseDetected()
}

type nopObserver struct{}

func (nopObserver) LoginFailure()         {}
func (nopObserver) RefreshReuseDetected() {}

func NewNopObserver() Observer {
	return nopObserver{}
}
