package mail

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
)

// @func sendTemplateEmail
// @description Go 템플릿으로 HTML 이메일을 발송한다

type SendTemplateEmailRequest struct {
	Host         string
	Port         int
	Username     string
	Password     string
	From         string
	To           string
	Subject      string
	TemplateName string            // 템플릿 파일 경로 또는 인라인 템플릿
	Data         map[string]string // 템플릿 변수
}

type SendTemplateEmailResponse struct{}

func SendTemplateEmail(req SendTemplateEmailRequest) (SendTemplateEmailResponse, error) {
	tmpl, err := template.New("email").Parse(req.TemplateName)
	if err != nil {
		return SendTemplateEmailResponse{}, err
	}
	var body bytes.Buffer
	if err := tmpl.Execute(&body, req.Data); err != nil {
		return SendTemplateEmailResponse{}, err
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n%s",
		req.From, req.To, req.Subject, body.String())
	auth := smtp.PlainAuth("", req.Username, req.Password, req.Host)
	addr := fmt.Sprintf("%s:%d", req.Host, req.Port)
	err = smtp.SendMail(addr, auth, req.From, []string{req.To}, []byte(msg))
	return SendTemplateEmailResponse{}, err
}
