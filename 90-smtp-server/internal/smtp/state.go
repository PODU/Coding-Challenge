package smtp

// State is where a session is in the SMTP command sequence.
type State int

const (
	StateGreet   State = iota // connected, no HELO/EHLO yet
	StateGreeted              // HELO/EHLO done, no transaction
	StateMail                 // MAIL FROM accepted
	StateRcpt                 // at least one RCPT TO accepted
	StateData                 // reading the message body
)

func (s State) String() string {
	switch s {
	case StateGreet:
		return "Greet"
	case StateGreeted:
		return "Greeted"
	case StateMail:
		return "Mail"
	case StateRcpt:
		return "Rcpt"
	case StateData:
		return "Data"
	default:
		return "Unknown"
	}
}
