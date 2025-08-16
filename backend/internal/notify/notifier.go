package notify

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"familyvault/internal/config"
)

// Notifier interface for sending notifications
type Notifier interface {
	Send(to, subject, body string) error
}

// NotificationService manages multiple notification channels
type NotificationService struct {
	email EmailNotifier
	sms   SMSNotifier
}

// NewNotificationService creates a new notification service
func NewNotificationService(cfg *config.Config) *NotificationService {
	return &NotificationService{
		email: NewEmailNotifier(cfg.SMTP),
		sms:   NewSMSNotifier(cfg.SMS),
	}
}

// SendEmail sends an email notification
func (ns *NotificationService) SendEmail(to, subject, body string) error {
	return ns.email.Send(to, subject, body)
}

// SendSMS sends an SMS notification
func (ns *NotificationService) SendSMS(to, body string) error {
	return ns.sms.Send(to, "", body)
}

// SendMultiChannel sends notifications via multiple channels
func (ns *NotificationService) SendMultiChannel(to, subject, body string, channels []string) (int, int) {
	sent := 0
	failed := 0

	for _, channel := range channels {
		var err error
		switch strings.ToLower(channel) {
		case "email":
			err = ns.SendEmail(to, subject, body)
		case "sms":
			err = ns.SendSMS(to, body)
		default:
			failed++
			continue
		}

		if err != nil {
			failed++
		} else {
			sent++
		}
	}

	return sent, failed
}

// EmailNotifier handles email notifications
type EmailNotifier struct {
	config config.SMTPConfig
}

// NewEmailNotifier creates a new email notifier
func NewEmailNotifier(cfg config.SMTPConfig) EmailNotifier {
	return EmailNotifier{config: cfg}
}

// Send sends an email notification
func (e EmailNotifier) Send(to, subject, body string) error {
	if e.config.Host == "" || e.config.Port == 0 {
		return fmt.Errorf("SMTP not configured")
	}

	// Create message
	msg := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", to, subject, body)

	// Setup authentication
	auth := smtp.PlainAuth("", e.config.Username, e.config.Password, e.config.Host)

	// Setup TLS config
	var err error
	if e.config.TLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         e.config.Host,
		}

		conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", e.config.Host, e.config.Port), tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to connect with TLS: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, e.config.Host)
		if err != nil {
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}
		defer client.Quit()

		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}

		if err = client.Mail(e.config.From); err != nil {
			return fmt.Errorf("failed to set sender: %w", err)
		}

		if err = client.Rcpt(to); err != nil {
			return fmt.Errorf("failed to set recipient: %w", err)
		}

		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("failed to get data writer: %w", err)
		}

		_, err = w.Write([]byte(msg))
		if err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}

		err = w.Close()
		if err != nil {
			return fmt.Errorf("failed to close data writer: %w", err)
		}

		return nil
	} else {
		// Non-TLS connection
		addr := fmt.Sprintf("%s:%d", e.config.Host, e.config.Port)
		err = smtp.SendMail(addr, auth, e.config.From, []string{to}, []byte(msg))
		if err != nil {
			return fmt.Errorf("failed to send email: %w", err)
		}
		return nil
	}
}

// SMSNotifier handles SMS notifications
type SMSNotifier struct {
	config config.SMSConfig
}

// NewSMSNotifier creates a new SMS notifier
func NewSMSNotifier(cfg config.SMSConfig) SMSNotifier {
	return SMSNotifier{config: cfg}
}

// Send sends an SMS notification
func (s SMSNotifier) Send(to, subject, body string) error {
	if s.config.Provider == "none" || s.config.Provider == "" {
		// No-op implementation when SMS is not configured
		return nil
	}

	if s.config.Provider == "twilio" {
		return s.sendTwilio(to, body)
	}

	return fmt.Errorf("unsupported SMS provider: %s", s.config.Provider)
}

// sendTwilio sends SMS via Twilio (stub implementation)
func (s SMSNotifier) sendTwilio(to, body string) error {
	if s.config.AccountSID == "" || s.config.AuthToken == "" {
		return fmt.Errorf("Twilio credentials not configured")
	}

	// TODO: Implement actual Twilio API call
	// For now, this is a stub that always succeeds
	// In a real implementation, you would use the Twilio REST API
	// to send the SMS message

	return nil // Stub: always succeeds
}
