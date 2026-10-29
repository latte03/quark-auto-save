package notify

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

func smtpMail(_ *Sender, ctx context.Context, cfg Config, title, content string) error {
	server := cfg.Str("SMTP_SERVER")
	host, _, err := net.SplitHostPort(server)
	if err != nil {
		host = server
	}
	from := cfg.Str("SMTP_EMAIL")
	password := cfg.Str("SMTP_PASSWORD")
	useSSL := cfg.Truthy("SMTP_SSL")

	toList := []string{from}
	if raw := cfg.Str("SMTP_EMAIL_TO"); raw != "" {
		toList = splitList(raw)
	}
	names := splitList(cfg.Str("SMTP_NAME_TO"))
	fromName := cfg.Str("SMTP_NAME")

	fromAddress := (&mail.Address{Name: fromName, Address: from}).String()
	toAddresses := make([]string, 0, len(toList))
	for i, address := range toList {
		name := ""
		if i < len(names) {
			name = names[i]
		}
		toAddresses = append(toAddresses, (&mail.Address{Name: name, Address: address}).String())
	}

	message := buildMail(fromAddress, strings.Join(toAddresses, ", "), title, content)

	dialer := &net.Dialer{Timeout: defaultTimeout}
	var conn net.Conn
	if useSSL {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: &tls.Config{ServerName: host}}).DialContext(ctx, "tcp", server)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", server)
	}
	if err != nil {
		return fmt.Errorf("连接 %s 失败: %w", server, err)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return err
	}
	defer client.Close()
	if err := client.Auth(smtp.PlainAuth("", from, password, host)); err != nil {
		return fmt.Errorf("SMTP 认证失败: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, address := range toList {
		if err := client.Rcpt(address); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write([]byte(message)); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func buildMail(from, to, subject, content string) string {
	var builder strings.Builder
	builder.WriteString("From: " + from + "\r\n")
	builder.WriteString("To: " + to + "\r\n")
	builder.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	builder.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	builder.WriteString("MIME-Version: 1.0\r\n")
	builder.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	builder.WriteString("Content-Transfer-Encoding: base64\r\n")
	builder.WriteString("\r\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	for len(encoded) > 76 {
		builder.WriteString(encoded[:76] + "\r\n")
		encoded = encoded[76:]
	}
	builder.WriteString(encoded + "\r\n")
	return builder.String()
}
