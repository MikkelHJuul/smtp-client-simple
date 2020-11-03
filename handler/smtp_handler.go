package handler

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"strings"
)

type mail struct {
	from, subject, body string
	to                  []string
}

func (m *mail) build(lsep string) string {
	var (
		fromTo  = ""
		subject = ""
		body    = ""
	)
	fromTo = fmt.Sprintf("From: %s%sTo: %s%s", m.from, lsep, strings.Join(m.to, ","), lsep)
	if m.subject != "" {
		subject = fmt.Sprintf("Subject: %s%s", m.subject, lsep)
	}
	if m.body != "" {
		body = fmt.Sprintf("%s%s", lsep, m.body)
	}
	return fromTo + subject + body
}

type SmtpHandler struct {
	smtpAddr   string
	lockedFrom string
	defaults   map[string]string
	skipTls    bool
}

func (s *SmtpHandler) reqFieldOrDefault(req *http.Request, field string) string {
	if f := req.FormValue(field); f != "" {
		return f
	}
	return s.defaults[field]
}

func (s *SmtpHandler) reqFieldsOrDefault(req *http.Request, field string) []string {
	if f := req.Form[field]; len(f) != 0 {
		return f
	}
	return strings.Split(s.defaults[field], ",")
}

func (s *SmtpHandler) newMailFromRequest(req *http.Request) (*mail, string, error) {
	var body string
	if req.Method == http.MethodPost {
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, "", err
		}
		log.Print("Got mail message body from POST.")
		body = string(b)
	}
	if err := req.ParseForm(); err != nil {
		return nil, "", err
	}

	m := &mail{
		from:    s.reqFieldOrDefault(req, "from"),
		to:      s.reqFieldsOrDefault(req, "to"),
		subject: s.reqFieldOrDefault(req, "subject"),
		body:    s.reqFieldOrDefault(req, "msg"),
	}

	if s.lockedFrom != "" {
		m.from = s.lockedFrom
		//From/To in the mail body may be set differently - how does that work?
	}

	if len(m.to) == 0 || m.from == "" {
		return nil, "", errors.New("missing fields in mail. Set appropriate parameters: to, from")
	}

	return m, body, nil
}

func respondError(wr http.ResponseWriter, err error) {
	wr.Header().Add("Content-Type", "text/plain")
	wr.WriteHeader(400)
	// Careful here about leaking info to an attacker...
	_, _ = fmt.Fprintf(wr, err.Error())
}

func respondOk(wr http.ResponseWriter, m *mail) {
	wr.Header().Add("Content-Type", "text/plain")
	wr.WriteHeader(200)

	_, _ = fmt.Fprintf(wr, m.build("\n"))
}

func (s *SmtpHandler) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	log.Print(req.RemoteAddr + " | " + req.Method + " " + req.URL.String())

	m, postBody, err := s.newMailFromRequest(req)
	if err != nil {
		respondError(wr, err)
		return
	}
	var body []byte
	if postBody != "" {
		body = []byte(postBody)
	} else {
		body = []byte(m.build("\n"))
	}
	if s.skipTls {
		err = s.SendMail(s.smtpAddr, m.from, m.to, body)
	} else {
		err = smtp.SendMail(s.smtpAddr, nil, m.from, m.to, body)
	}
	if err != nil {
		log.Printf("got error from mail send-method: %s", err.Error())
		respondError(wr, err)
	} else {
		respondOk(wr, m)
	}
}

func (s *SmtpHandler) SendMail(addr string, from string, to []string, data []byte) error {
	c, err := smtp.Dial(addr)
	if err != nil {
		log.Print(err)
	}

	if err := c.Mail(from); err != nil {
		return err
	}

	for _, mail := range to {
		if err := c.Rcpt(mail); err != nil {
			return err
		}
	}

	// Send the email body.
	wc, err := c.Data()
	if err != nil {
		return err
	}
	if _, err = wc.Write(data); err != nil {
		return err
	}

	if err = wc.Close(); err != nil {
		return err
	}

	// Send the QUIT command and close the connection.
	if err = c.Quit(); err != nil {
		return err
	}
	return nil
}
