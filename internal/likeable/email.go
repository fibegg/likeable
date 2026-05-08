package likeable

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type smtpSettings struct {
	Host      string
	Port      string
	Username  string
	Password  string
	FromEmail string
	FromName  string
	TLSMode   string
}

type emailMessage struct {
	To      string
	Subject string
	Body    string
}

type emailSender interface {
	Send(context.Context, smtpSettings, emailMessage) error
}

type smtpEmailSender struct{}

func (s *Server) emailSettings(ctx context.Context) (smtpSettings, bool, error) {
	cfg, err := s.store.ConfigMap(ctx)
	if err != nil {
		return smtpSettings{}, false, err
	}
	settings := smtpSettings{
		Host:      strings.TrimSpace(cfg["smtp_host"]),
		Port:      firstNonEmpty(cfg["smtp_port"], "587"),
		Username:  strings.TrimSpace(cfg["smtp_username"]),
		Password:  strings.TrimSpace(cfg["smtp_password"]),
		FromEmail: strings.TrimSpace(cfg["smtp_from_email"]),
		FromName:  firstNonEmpty(cfg["smtp_from_name"], "Likeable"),
		TLSMode:   normalizeSMTPTLSMode(firstNonEmpty(cfg["smtp_tls_mode"], "auto")),
	}
	if settings.Host == "" || settings.FromEmail == "" {
		return settings, false, nil
	}
	if _, err := strconv.Atoi(settings.Port); err != nil {
		return settings, false, fmt.Errorf("smtp_port must be a number")
	}
	if _, err := mail.ParseAddress(settings.FromEmail); err != nil {
		return settings, false, fmt.Errorf("smtp_from_email is invalid: %w", err)
	}
	return settings, true, nil
}

func normalizeSMTPTLSMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "tls", "ssl", "implicit_tls":
		return "tls"
	case "starttls", "start_tls":
		return "starttls"
	case "none", "plain", "off", "disabled":
		return "none"
	default:
		return "auto"
	}
}

func (s *Server) sendUserEmailAsync(to, subject, body string) {
	s.sendEmailAsync(to, subject, body)
}

func (s *Server) sendAdminEmailAsync(subject, body string) {
	if s.config.AdminEmail == "" {
		return
	}
	s.sendEmailAsync(s.config.AdminEmail, subject, body)
}

func (s *Server) sendEmailAsync(to, subject, body string) {
	to = normalizeEmail(to)
	subject = strings.TrimSpace(subject)
	body = strings.TrimSpace(body)
	if to == "" || subject == "" || body == "" {
		return
	}
	if s.jobs != nil {
		if err := s.enqueueEmailJob(context.Background(), emailJobPayload{To: to, Subject: subject, Body: body}); err != nil {
			log.Printf("enqueue email to %s: %v", to, err)
		}
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := s.sendEmail(ctx, emailMessage{To: to, Subject: subject, Body: body}); err != nil {
			log.Printf("send email to %s: %v", to, err)
		}
	}()
}

func (s *Server) sendEmail(ctx context.Context, message emailMessage) error {
	settings, ok, err := s.emailSettings(ctx)
	if err != nil || !ok {
		return err
	}
	sender := s.email
	if sender == nil {
		sender = smtpEmailSender{}
	}
	return sender.Send(ctx, settings, message)
}

func (smtpEmailSender) Send(ctx context.Context, settings smtpSettings, message emailMessage) error {
	toAddress, err := mail.ParseAddress(message.To)
	if err != nil {
		return err
	}
	fromAddress := mail.Address{Name: settings.FromName, Address: settings.FromEmail}
	addr := net.JoinHostPort(settings.Host, settings.Port)
	dialer := &net.Dialer{Timeout: 12 * time.Second}
	var conn net.Conn
	if settings.TLSMode == "tls" {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: settings.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(35 * time.Second))
	client, err := smtp.NewClient(conn, settings.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer client.Close()
	if settings.TLSMode == "starttls" || settings.TLSMode == "auto" {
		ok, _ := client.Extension("STARTTLS")
		if ok {
			if err := client.StartTLS(&tls.Config{ServerName: settings.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return err
			}
		} else if settings.TLSMode == "starttls" {
			return fmt.Errorf("SMTP server does not advertise STARTTLS")
		}
	}
	if settings.Username != "" || settings.Password != "" {
		if err := client.Auth(smtp.PlainAuth("", settings.Username, settings.Password, settings.Host)); err != nil {
			return err
		}
	}
	if err := client.Mail(settings.FromEmail); err != nil {
		return err
	}
	if err := client.Rcpt(toAddress.Address); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := io.WriteString(writer, buildEmailMessage(fromAddress.String(), toAddress.String(), message.Subject, message.Body)); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func buildEmailMessage(from, to, subject, body string) string {
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + time.Now().UTC().Format(time.RFC1123Z),
		"Message-ID: <" + uuid.NewString() + "@likeable.local>",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: 8bit",
	}
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + strings.TrimSpace(body) + "\r\n"
}

func (s *Server) systemMessageEmailBody(user *User, body string) string {
	return fmt.Sprintf("Hi %s,\n\nLikeable sent you a system message:\n\n%s\n\nOpen your profile messages:\n%s\n", displayEmailName(user), body, s.profileURL())
}

func supportEmailBody(user *User, body string) string {
	return fmt.Sprintf("A Likeable user sent a support message.\n\nUser: %s\nUser ID: %s\n\nMessage:\n%s\n", user.Email, user.ID, body)
}

func displayEmailName(user *User) string {
	if user == nil {
		return "there"
	}
	if strings.TrimSpace(user.Name) != "" {
		return strings.TrimSpace(user.Name)
	}
	if user.Email != "" {
		return user.Email
	}
	return "there"
}
