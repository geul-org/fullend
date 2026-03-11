package mail

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
	"strconv"
)

// @func sendTemplateEmail
// @description Go 템플릿으로 HTML 이메일을 발송한다

type SendTemplateEmailRequest struct {
	To           string
	Subject      string
	TemplateName string
}

type SendTemplateEmailResponse struct{}

func SendTemplateEmail(req SendTemplateEmailRequest) (SendTemplateEmailResponse, error) {
	host := os.Getenv("SMTP_HOST")
	port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	from := os.Getenv("SMTP_FROM")

	tmpl, err := template.New("email").Parse(req.TemplateName)
	if err != nil {
		return SendTemplateEmailResponse{}, err
	}
	var body bytes.Buffer
	if err := tmpl.Execute(&body, nil); err != nil {
		return SendTemplateEmailResponse{}, err
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n%s",
		from, req.To, req.Subject, body.String())
	auth := smtp.PlainAuth("", username, password, host)
	addr := fmt.Sprintf("%s:%d", host, port)
	err = smtp.SendMail(addr, auth, from, []string{req.To}, []byte(msg))
	return SendTemplateEmailResponse{}, err
}
