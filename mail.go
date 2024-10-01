package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
	"time"
)

type unencryptedAuth struct {
	smtp.Auth
}

func (a unencryptedAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	s := *server
	s.TLS = true
	return a.Auth.Start(&s)
}

func setupClient(host, port, username, password string, allowInsecure bool) (*smtp.Client, error) {
	var auth smtp.Auth
	auth = smtp.PlainAuth("", username, password, host)
	if port == "25" {
		if !allowInsecure {
			return nil, errors.New("sending plain password over unencrypted connection")
		}
		auth = unencryptedAuth{auth}
	}

	var tlsconfig *tls.Config
	if port != "25" {
		// TLS config
		tlsconfig = &tls.Config{
			InsecureSkipVerify: allowInsecure,
			ServerName:         host,
		}
	}

	servername := host + ":" + port

	var c *smtp.Client
	var err error
	if port == "465" {
		// Here is the key, you need to call tls.Dial instead of smtp.Dial
		// for smtp servers running on 465 that require an ssl connection
		// from the very beginning (no starttls)
		conn, err := tls.Dial("tcp", servername, tlsconfig)
		if err != nil {
			return nil, err
		}

		c, err = smtp.NewClient(conn, host)
		if err != nil {
			return nil, err
		}
	} else {
		c, err = smtp.Dial(servername)
		if err != nil {
			return nil, err
		}
		if port == "587" {
			c.StartTLS(tlsconfig)
		}
	}

	// Auth
	if err = c.Auth(auth); err != nil {
		fmt.Println("here", err)
		return nil, err
	}

	return c, nil
}

func sendMail(from, to, subject, body string, c *smtp.Client) error {
	// Setup headers
	headers := make(map[string]string)
	headers["From"] = from
	recipients := strings.Split(to, ",")
	recipientsStr := make([]string, 0)
	for i := range recipients {
		recipientsStr = append(recipientsStr, fmt.Sprintf("<%s>", recipients[i]))
	}
	headers["To"] = strings.Join(recipientsStr, ",")
	headers["Subject"] = subject

	// Setup message
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// To && From
	if err := c.Mail(from); err != nil {
		return err
	}

	if err := c.Rcpt(strings.Join(recipientsStr, ",")); err != nil {
		return err
	}

	// Data
	w, err := c.Data()
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(message))
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return nil
}

type mailCtx struct {
	NewChunks    uint64
	ReusedChunks uint64
	Datastore    string
	Error        error
	Hostname     string
	StartTime    time.Time
	EndTime      time.Time
}

func (m *mailCtx) Duration() time.Duration {
	return m.EndTime.Sub(m.StartTime)
}

func (m *mailCtx) FromattedDuration() string {
	return m.Duration().String()
}

func (m *mailCtx) ErrorStr() string {
	if m.Error != nil {
		return m.Error.Error()
	}
	return ""
}

func (m *mailCtx) Success() bool {
	return m.Error == nil
}

func (m *mailCtx) Status() string {
	if m.Success() {
		return "Success"
	}
	return "Failed"
}

func (m *mailCtx) buildStr(txt string) (string, error) {
	tmpl, err := template.New("mail").Parse(txt)
	if err != nil {
		return "", err
	}
	strBuff := &strings.Builder{}
	err = tmpl.Execute(strBuff, m)
	return strBuff.String(), err
}
