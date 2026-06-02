package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"os"
)

func sendVerificationEmail(toEmail, username, token string) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	smtpFrom := os.Getenv("SMTP_FROM")

	// If SMTP is not configured, just log the token (useful for local dev without SMTP)
	if smtpHost == "" || smtpPort == "" {
		log.Printf("SMTP not configured. Verification code for %s: %s", toEmail, token)
		return nil
	}
	if smtpFrom == "" {
		smtpFrom = smtpUser
	}

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         smtpHost,
	}

	conn, err := tls.Dial("tcp", smtpHost+":"+smtpPort, tlsconfig)
	if err != nil {
		log.Printf("Failed to dial SMTP server: %v", err)
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		log.Printf("Failed to create SMTP client: %v", err)
		return err
	}

	if err = client.Auth(auth); err != nil {
		log.Printf("SMTP Auth failed: %v", err)
		return err
	}

	if err = client.Mail(smtpFrom); err != nil {
		return err
	}

	if err = client.Rcpt(toEmail); err != nil {
		return err
	}

	w, err := client.Data()
	if err != nil {
		return err
	}

	subject := "Verify your FH6 Telemetry Account"
	body := fmt.Sprintf("Hello %s,\n\nYour verification code is: %s\n\nPlease enter this code on the verification page to activate your account.\n\nThanks,\nFH6 Telemetry Team", username, token)

	msg := []byte(fmt.Sprintf("To: %s\r\nFrom: %s\r\nSubject: %s\r\n\r\n%s", toEmail, smtpFrom, subject, body))
	_, err = w.Write(msg)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}
