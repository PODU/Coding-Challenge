package smtp

import "time"

// Envelope is a received message, as handed to the MessageStore.
type Envelope struct {
	From     string
	To       []string
	Data     []byte // body with dot-stuffing undone and the final "." removed
	Received time.Time
}

// Transaction is the mail transaction in progress. RSET, HELO/EHLO and a
// stored message all reset it.
type Transaction struct {
	From    string
	To      []string
	started bool
}

func (t *Transaction) reset() {
	t.From = ""
	t.To = nil
	t.started = false
}

// Started reports whether a MAIL FROM has begun a transaction.
func (t *Transaction) Started() bool { return t.started }

func (t *Transaction) begin(from string) {
	t.From = from
	t.To = nil
	t.started = true
}

func (t *Transaction) addRecipient(addr string) {
	t.To = append(t.To, addr)
}
