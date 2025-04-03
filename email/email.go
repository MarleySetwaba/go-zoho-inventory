package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
)

type UserEmail struct {
	Subject string `json:"subject"`
	User    User   `json:"user"`
	Parts   []Part `json:"parts"`
}

type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	ContactName string `json:"contact_name"`
}

type Part struct {
	SKU         string  `json:"sku"`
	Title       string  `json:"title"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
}

func SendEmail(to string, userEmail UserEmail) error {
	from := "dancemediainc@gmail.com"
	password := "txjv olva isgi xgnr"
	smtpServer := "smtp.gmail.com"
	smtpPort := "587"

	auth := smtp.PlainAuth("", from, password, smtpServer)

	t, err := template.ParseFiles("./email/email_template.html")
	if err != nil {
		fmt.Println(err)
		return err
	}

	var body bytes.Buffer
	err = t.Execute(&body, userEmail)
	if err != nil {

		return err
	}

	msg := []byte("Subject: " + userEmail.Subject + "\r\n" +
		"From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"MIME-version: 1.0;\r\n" +
		"Content-Type: text/html; charset=\"UTF-8\";\r\n" +
		"\r\n" + body.String())

	err = smtp.SendMail(smtpServer+":"+smtpPort, auth, from, []string{to}, msg)
	if err != nil {
		fmt.Println(err)
		return err
	}

	fmt.Println("Email Sent Successfully")
	return nil
}
