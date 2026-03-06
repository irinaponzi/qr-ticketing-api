package adapter

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
)

// SMTPEmailSender sends ticket emails via SMTP.
// When user and password are empty, it sends without authentication (MailHog mode).
// When user and password are set, it uses STARTTLS + PLAIN authentication.
type SMTPEmailSender struct {
	host     string
	port     int
	from     string
	user     string
	password string
	logger   *slog.Logger
}

// NewSMTPEmailSender creates a new SMTPEmailSender configured for the given SMTP server.
//
// Parameters:
//   - host: The SMTP server hostname (e.g., "localhost" for MailHog, "smtp.gmail.com" for Gmail).
//   - port: The SMTP server port (e.g., 1025 for MailHog, 587 for TLS).
//   - from: The sender email address.
//   - user: The SMTP username for authentication. Empty string disables auth.
//   - password: The SMTP password for authentication. Empty string disables auth.
//   - logger: Structured logger for observability.
//
// Returns:
//   - *SMTPEmailSender: A pointer to the newly created sender.
func NewSMTPEmailSender(host string, port int, from, user, password string, logger *slog.Logger) *SMTPEmailSender {
	return &SMTPEmailSender{
		host:     host,
		port:     port,
		from:     from,
		user:     user,
		password: password,
		logger:   logger,
	}
}

// SendTicketEmail sends a ticket confirmation email with QR code images.
// Each QR image is embedded inline in the HTML body (via CID) and also
// attached as a PNG file for clients that don't render inline images.
//
// Parameters:
//   - ctx: The request context for cancellation and tracing.
//   - to: The recipient email address.
//   - eventName: The name of the event for the email subject.
//   - qrImages: The QR code PNG images to embed and attach.
//
// Returns:
//   - error: A wrapped error if sending fails; otherwise, nil.
func (s *SMTPEmailSender) SendTicketEmail(ctx context.Context, to string, eventName string, qrImages [][]byte) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	msg, err := s.buildMIMEMessage(to, eventName, qrImages)
	if err != nil {
		return fmt.Errorf("building email message: %w", err)
	}

	if err := s.sendMail(addr, to, []byte(msg)); err != nil {
		return fmt.Errorf("sending email to %s: %w", to, err)
	}

	s.logger.InfoContext(ctx, "ticket email sent", "to", to, "event", eventName, "tickets", len(qrImages))

	return nil
}

// buildMIMEMessage constructs a multipart/mixed MIME email with:
//   - A multipart/related section containing the HTML body and inline QR images.
//   - Each QR image also attached as a standalone PNG file.
func (s *SMTPEmailSender) buildMIMEMessage(to, eventName string, qrImages [][]byte) (string, error) {
	var buf strings.Builder

	mixedWriter := multipart.NewWriter(&buf)

	// Top-level headers.
	buf.Reset()
	fmt.Fprintf(&buf, "From: %s\r\n", s.from)
	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "Subject: Your tickets for %s\r\n", eventName)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=%q\r\n", mixedWriter.Boundary())
	fmt.Fprintf(&buf, "\r\n")

	// --- HTML body with inline images ---
	relatedWriter := multipart.NewWriter(nil)

	htmlHeader := make(textproto.MIMEHeader)
	htmlHeader.Set("Content-Type", fmt.Sprintf("multipart/related; boundary=%q", relatedWriter.Boundary()))

	relatedPart, err := mixedWriter.CreatePart(htmlHeader)
	if err != nil {
		return "", fmt.Errorf("creating related part: %w", err)
	}

	relatedWriterOnPart := multipart.NewWriter(relatedPart)
	relatedWriterOnPart.SetBoundary(relatedWriter.Boundary())

	// HTML part.
	htmlPartHeader := make(textproto.MIMEHeader)
	htmlPartHeader.Set("Content-Type", "text/html; charset=UTF-8")

	htmlPart, err := relatedWriterOnPart.CreatePart(htmlPartHeader)
	if err != nil {
		return "", fmt.Errorf("creating html part: %w", err)
	}

	var htmlBody strings.Builder
	fmt.Fprintf(&htmlBody, "<html><body>")
	fmt.Fprintf(&htmlBody, "<h2>Your tickets for %s</h2>", eventName)
	fmt.Fprintf(&htmlBody, "<p>You have %d ticket(s). Present these QR codes at the venue entrance:</p>", len(qrImages))

	for i := range qrImages {
		cid := fmt.Sprintf("qr-ticket-%d", i+1)
		fmt.Fprintf(&htmlBody, "<div style=\"margin: 16px 0;\">")
		fmt.Fprintf(&htmlBody, "<p><strong>Ticket %d</strong></p>", i+1)
		fmt.Fprintf(&htmlBody, "<img src=\"cid:%s\" alt=\"QR Code Ticket %d\" width=\"256\" height=\"256\" />", cid, i+1)
		fmt.Fprintf(&htmlBody, "</div>")
	}

	fmt.Fprintf(&htmlBody, "<p>Enjoy the event!</p>")
	fmt.Fprintf(&htmlBody, "</body></html>")
	fmt.Fprint(htmlPart, htmlBody.String())

	// Inline QR images (CID references).
	for i, img := range qrImages {
		cid := fmt.Sprintf("qr-ticket-%d", i+1)

		imgHeader := make(textproto.MIMEHeader)
		imgHeader.Set("Content-Type", "image/png")
		imgHeader.Set("Content-Transfer-Encoding", "base64")
		imgHeader.Set("Content-ID", fmt.Sprintf("<%s>", cid))
		imgHeader.Set("Content-Disposition", "inline")

		imgPart, err := relatedWriterOnPart.CreatePart(imgHeader)
		if err != nil {
			return "", fmt.Errorf("creating inline image part %d: %w", i, err)
		}

		fmt.Fprint(imgPart, base64.StdEncoding.EncodeToString(img))
	}

	if err := relatedWriterOnPart.Close(); err != nil {
		return "", fmt.Errorf("closing related writer: %w", err)
	}

	// --- Attached QR images as separate PNG files ---
	for i, img := range qrImages {
		attHeader := make(textproto.MIMEHeader)
		attHeader.Set("Content-Type", "image/png")
		attHeader.Set("Content-Transfer-Encoding", "base64")
		attHeader.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"ticket-%d.png\"", i+1))

		attPart, err := mixedWriter.CreatePart(attHeader)
		if err != nil {
			return "", fmt.Errorf("creating attachment part %d: %w", i, err)
		}

		fmt.Fprint(attPart, base64.StdEncoding.EncodeToString(img))
	}

	if err := mixedWriter.Close(); err != nil {
		return "", fmt.Errorf("closing mixed writer: %w", err)
	}

	return buf.String(), nil
}

// sendMail sends the message using plain SMTP (no auth) or STARTTLS + PLAIN auth
// depending on whether credentials are configured.
func (s *SMTPEmailSender) sendMail(addr, to string, msg []byte) error {
	if s.user == "" {
		return smtp.SendMail(addr, nil, s.from, []string{to}, msg)
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("connecting to SMTP server: %w", err)
	}

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("creating SMTP client: %w", err)
	}
	defer client.Close()

	tlsConfig := &tls.Config{ServerName: s.host}
	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("starting TLS: %w", err)
	}

	auth := smtp.PlainAuth("", s.user, s.password, s.host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication: %w", err)
	}

	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("SMTP RCPT TO: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("writing message: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("closing message writer: %w", err)
	}

	return client.Quit()
}
