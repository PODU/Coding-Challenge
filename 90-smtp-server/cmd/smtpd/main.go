// Command smtpd is a small SMTP server that saves each accepted message as a
// file in a maildir-style directory.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codingchallenges/smtpd/internal/smtp"
)

func main() {
	addr := flag.String("addr", ":2525", "listen address host:port (use :25 for standard SMTP)")
	maildir := flag.String("maildir", "./maildir", "directory to store received messages")
	hostname := flag.String("hostname", "", "server hostname announced in greeting/EHLO (default OS hostname)")
	timeout := flag.Duration("timeout", 5*time.Minute, "idle read timeout per connection")
	flag.Parse()

	store, err := smtp.NewMaildirStore(*maildir)
	if err != nil {
		log.Fatalf("smtpd: %v", err)
	}

	srv := smtp.NewServer(*hostname, store, *timeout)
	if err := srv.Listen(*addr); err != nil {
		log.Fatalf("smtpd: listen: %v", err)
	}
	log.Printf("smtpd: listening on %s, storing mail in %s", srv.Addr(), *maildir)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Printf("smtpd: shutting down, draining connections...")
	}()

	if err := srv.Serve(ctx); err != nil {
		log.Fatalf("smtpd: serve: %v", err)
	}
	log.Printf("smtpd: stopped")
}
