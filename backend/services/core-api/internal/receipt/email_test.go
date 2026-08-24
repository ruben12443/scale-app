package receipt

import (
	"context"
	"io"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

// smtpMessage captures what a client sent to the fake SMTP server below.
type smtpMessage struct {
	From, To, Data string
}

// startFakeSMTPServer runs a minimal real SMTP server (no STARTTLS, no auth)
// so SMTPSender.Send is exercised against actual wire protocol behavior via
// net/smtp, not just a mocked interface.
func startFakeSMTPServer(t *testing.T) (addr string, received chan smtpMessage) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	received = make(chan smtpMessage, 1)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleFakeSMTPConn(conn, received)
		}
	}()
	return ln.Addr().String(), received
}

func handleFakeSMTPConn(conn net.Conn, received chan smtpMessage) {
	defer conn.Close()
	tp := textproto.NewConn(conn)

	if err := tp.PrintfLine("220 fake.smtp.local ESMTP"); err != nil {
		return
	}

	var msg smtpMessage
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			// Deliberately advertise no extensions (no STARTTLS, no AUTH) so
			// the client proceeds straight to MAIL/RCPT/DATA in plaintext.
			_ = tp.PrintfLine("250 fake.smtp.local")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			msg.From = strings.TrimSpace(line[len("MAIL FROM:"):])
			_ = tp.PrintfLine("250 OK")
		case strings.HasPrefix(upper, "RCPT TO:"):
			msg.To = strings.TrimSpace(line[len("RCPT TO:"):])
			_ = tp.PrintfLine("250 OK")
		case upper == "DATA":
			_ = tp.PrintfLine("354 Start mail input; end with <CRLF>.<CRLF>")
			data, err := io.ReadAll(tp.DotReader())
			if err != nil {
				return
			}
			msg.Data = string(data)
			_ = tp.PrintfLine("250 OK: queued")
			received <- msg
		case upper == "QUIT":
			_ = tp.PrintfLine("221 Bye")
			return
		default:
			_ = tp.PrintfLine("250 OK")
		}
	}
}

func TestSMTPSenderSend(t *testing.T) {
	addr, received := startFakeSMTPServer(t)

	sender := &SMTPSender{Addr: addr, From: "noreply@example.com"}
	err := sender.Send(context.Background(), "customer@example.com", "Your receipt", "<html><body>Thanks!</body></html>")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	select {
	case msg := <-received:
		if !strings.Contains(msg.From, "noreply@example.com") {
			t.Fatalf("MAIL FROM = %q, want it to contain noreply@example.com", msg.From)
		}
		if !strings.Contains(msg.To, "customer@example.com") {
			t.Fatalf("RCPT TO = %q, want it to contain customer@example.com", msg.To)
		}
		if !strings.Contains(msg.Data, "Subject: Your receipt") {
			t.Fatalf("message data missing Subject header; got:\n%s", msg.Data)
		}
		if !strings.Contains(msg.Data, "<html><body>Thanks!</body></html>") {
			t.Fatalf("message data missing HTML body; got:\n%s", msg.Data)
		}
		if !strings.Contains(msg.Data, "Content-Type: text/html") {
			t.Fatalf("message data missing HTML content type; got:\n%s", msg.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the fake SMTP server to receive a message")
	}
}

func TestFakeEmailSenderRecordsSentEmails(t *testing.T) {
	sender := &FakeEmailSender{}
	if err := sender.Send(context.Background(), "a@b.com", "Subject", "<p>Body</p>"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if len(sender.Sent) != 1 || sender.Sent[0].To != "a@b.com" {
		t.Fatalf("Sent = %+v, want one email to a@b.com", sender.Sent)
	}
}
