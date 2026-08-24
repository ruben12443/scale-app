package receipt

import (
	"context"
	"sync"
)

// SentEmail records one call to FakeEmailSender.Send.
type SentEmail struct {
	To, Subject, HTMLBody string
}

// FakeEmailSender is an in-memory EmailSender for tests.
type FakeEmailSender struct {
	mu   sync.Mutex
	Sent []SentEmail
}

func (f *FakeEmailSender) Send(ctx context.Context, to, subject, htmlBody string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Sent = append(f.Sent, SentEmail{To: to, Subject: subject, HTMLBody: htmlBody})
	return nil
}
