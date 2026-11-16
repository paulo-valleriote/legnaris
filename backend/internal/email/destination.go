package email

type Destination string

const (
	DestinationRetry30s   Destination = "retry_30s"
	DestinationRetry2m    Destination = "retry_2m"
	DestinationRetry5m    Destination = "retry_5m"
	DestinationDeadLetter Destination = "dead_letter"
)

func (d Destination) Valid() bool {
	switch d {
	case DestinationRetry30s, DestinationRetry2m, DestinationRetry5m, DestinationDeadLetter:
		return true
	default:
		return false
	}
}
