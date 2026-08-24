package receipt

import (
	"bytes"
	"context"
	"fmt"
	"net/smtp"
)

// EmailSender sends a receipt (as HTML) to a customer.
type EmailSender interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
}

// SMTPSender sends email via net/smtp. It has no context-cancellation
// support of its own — net/smtp.SendMail is a blocking stdlib call with no
// context-aware variant — so ctx is accepted for interface consistency but
// not actually used to cancel an in-flight send.
type SMTPSender struct {
	Addr string // "host:port"
	Auth smtp.Auth
	From string
}

func (s *SMTPSender) Send(ctx context.Context, to, subject, htmlBody string) error {
	msg := buildMIMEMessage(s.From, to, subject, htmlBody)
	if err := smtp.SendMail(s.Addr, s.Auth, s.From, []string{to}, msg); err != nil {
		return fmt.Errorf("receipt: send email: %w", err)
	}
	return nil
}

func buildMIMEMessage(from, to, subject, htmlBody string) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(htmlBody)
	return buf.Bytes()
}
