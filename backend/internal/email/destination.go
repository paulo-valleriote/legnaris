package email

// Destination names where a message goes. The broker adapter maps each name to
// the exchange and routing key it owns, so a routing key is spelled in exactly
// one place (WKR-09, WKR-11).
type Destination string

const (
	// DestinationRetry30s is the first retry: about half a minute.
	DestinationRetry30s Destination = "retry_30s"
	// DestinationRetry2m is the second retry: two minutes.
	DestinationRetry2m Destination = "retry_2m"
	// DestinationRetry5m is every later retry: five minutes.
	DestinationRetry5m Destination = "retry_5m"
	// DestinationDeadLetter is where a message that can never succeed is parked.
	DestinationDeadLetter Destination = "dead_letter"
)

// Valid reports whether d is one of the four names; the vocabulary is closed
// (WKR-09).
func (d Destination) Valid() bool {
	switch d {
	case DestinationRetry30s, DestinationRetry2m, DestinationRetry5m, DestinationDeadLetter:
		return true
	default:
		return false
	}
}
