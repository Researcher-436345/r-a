// Package mailer sends transactional auth emails over SMTP.
// With MAIL_ENABLED=false (default) emails are printed to stdout instead —
// convenient for local dev without Mailpit.
package mailer

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/centraluniversity/researcher/internal/platform/config"
)

// Message is a plain-text + HTML email.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// Mailer sends messages synchronously; callers decide about async delivery.
type Mailer struct {
	host    string
	port    string
	user    string
	pass    string
	from    string
	enabled bool
}

// FromConfig builds a Mailer from SMTP_* / MAIL_ENABLED settings.
func FromConfig(cfg config.Config) *Mailer {
	return New(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom, cfg.MailEnabled)
}

// New builds a Mailer from explicit settings.
func New(host, port, user, pass, from string, enabled bool) *Mailer {
	if from == "" {
		from = "researcher@localhost"
	}
	return &Mailer{host: host, port: port, user: user, pass: pass, from: from, enabled: enabled}
}

// Send delivers msg over SMTP, or prints it when mail is disabled.
// Delivery happens in a goroutine so callers never block on SMTP;
// failures are logged, not returned.
func (m *Mailer) Send(msg Message) error {
	go func() {
		if !m.enabled {
			slog.Info("mail disabled, printing email", "to", msg.To, "subject", msg.Subject, "text", msg.Text)
			return
		}
		if err := m.send(msg); err != nil {
			slog.Error("failed to send email", "to", msg.To, "subject", msg.Subject, "error", err)
		} else {
			slog.Info("email sent", "to", msg.To, "subject", msg.Subject)
		}
	}()
	return nil
}

func (m *Mailer) send(msg Message) error {
	addr := net.JoinHostPort(m.host, m.port)
	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	c, err := smtp.NewClient(conn, m.host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer conn.Close()

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: m.host}); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if m.user != "" {
		if err := c.Auth(smtp.PlainAuth("", m.user, m.pass, m.host)); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := c.Mail(m.from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	if err := c.Rcpt(msg.To); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(buildMessage(m.from, msg)); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close body: %w", err)
	}
	return c.Quit()
}

func buildMessage(from string, msg Message) []byte {
	boundary := "researcher-mail-boundary-42"
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", msg.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%s\r\n", boundary)
	fmt.Fprintf(&b, "\r\n")
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: text/plain; charset=utf-8\r\n\r\n")
	b.WriteString(msg.Text)
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: text/html; charset=utf-8\r\n\r\n")
	b.WriteString(msg.HTML)
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String())
}
