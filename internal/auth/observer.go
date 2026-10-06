package auth

// Observer receives auth security signals. It is an interface so the auth
// service does not depend on the metrics implementation; internal/platform
// backs it with Prometheus counters, and NewNopObserver backs tests.
type Observer interface {
	LoginFailure()
	RefreshReuseDetected()
}

type nopObserver struct{}

func (nopObserver) LoginFailure()         {}
func (nopObserver) RefreshReuseDetected() {}

// NewNopObserver returns an Observer that does nothing.
func NewNopObserver() Observer {
	return nopObserver{}
}
