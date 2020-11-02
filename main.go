package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strings"
)

var (
	port     = flag.Int("port", 8080, "Port to listen on.")
	smtpAddr = flag.String("smtp-server", "", "Default server:port for smtp communications.")
	defFrom  = flag.String("from", "", "Default sender address.")
	defTo    = flag.String("to", "", "Default recipient address.")
	defSubj  = flag.String("subject", "", "Default message subject.")
	defMsg   = flag.String("message", "", "Default mail text.")
	reqFrom  = flag.String("forced-from", "", "If set, this is the sender, regardless of request parameters.")
	//insecure  = flag.Bool("insecure", false, "using the insecure flag disables log in to the smtp-server")
	skipTls = flag.Bool("skip-tls", false, "skip tls in the transport to the smtp-server")
)

func main() {
	flag.Parse()

	log.Print("Initiating Handler")
	handler := SmtpHandler{
		smtpAddr:   *smtpAddr,
		lockedFrom: *reqFrom,
		defaults: map[string]string{
			"to":      *defTo,
			"from":    *defFrom,
			"subject": *defSubj,
			"body":    *defMsg,
		},
	}
	log.Printf("\t%s: %s", "smtp address", *smtpAddr)
	log.Printf("\t%s: %s", "locked mail-from", *reqFrom)
	log.Printf("\t%s: ", "mail-defaults")
	for k, v := range handler.defaults {
		log.Printf("\t\t%s: %s", k, v)
	}
	if *skipTls {
		log.Printf("\t%s", "WARN: this instance is running without tls")
	}

	log.Printf("Smtp server listening on port %d.\n", *port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), &handler); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

const (
	rfc822LSep = "\r\n"
	unixLSep   = "\n"
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
	fromTo = fmt.Sprintf("From: %s%sTo: %s%s", m.from, lsep, m.to, lsep)
	if m.subject != "" {
		subject = fmt.Sprintf("Subject: %s%s", m.subject, lsep)
	}
	if m.body != "" {
		body = fmt.Sprintf("%s%s", lsep, m.body)
	}
	return fromTo + subject + body
}

func (m *mail) String() string {
	return m.build(unixLSep)
}

func (m *mail) ForData() []byte {
	return []byte(m.build(rfc822LSep) + fmt.Sprintf("%s.%s", rfc822LSep, rfc822LSep))
}

type SmtpHandler struct {
	smtpAddr   string
	lockedFrom string
	defaults   map[string]string
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

func (s *SmtpHandler) newMailFromRequest(req *http.Request) (*mail, error) {
	var body string
	if req.Method == http.MethodPost {
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		log.Print("Got mail message body from POST.")
		body = string(b)
	}

	m := &mail{
		to:      s.reqFieldsOrDefault(req, "to"),
		from:    s.reqFieldOrDefault(req, "from"),
		subject: s.reqFieldOrDefault(req, "subject"),
		body:    s.reqFieldOrDefault(req, "msg"),
	}

	if body != "" {
		log.Print("Using mail body from POST.")
		m.body = body
		if strings.Contains(body, "Subject: ") {
			m.subject = ""
		}
	}

	if s.lockedFrom != "" {
		m.from = s.lockedFrom
		//From/To in the mail body may be set differently - how does that work?
	}

	if len(m.to) == 0 || m.from == "" {
		return nil, errors.New("missing fields in mail. Set appropriate parameters: to, from")
	}

	return m, nil
}

func respondError(wr http.ResponseWriter, err error) {
	wr.Header().Add("Content-Type", "text/plain")
	wr.WriteHeader(400)
	// Careful here about leaking info to an attacker...
	_, _ = wr.Write([]byte(err.Error()))
}

func respondOk(wr http.ResponseWriter, m *mail) {
	wr.Header().Add("Content-Type", "text/plain")
	wr.WriteHeader(200)

	_, _ = wr.Write([]byte(m.String()))
}

func (s *SmtpHandler) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	log.Print(req.RemoteAddr + " | " + req.Method + " " + req.URL.String())

	m, err := s.newMailFromRequest(req)
	if err != nil {
		respondError(wr, err)
	}
	if *skipTls {
		err = s.SendMail(s.smtpAddr, m.from, m.to, m.ForData())
	} else {
		err = smtp.SendMail(s.smtpAddr, nil, m.from, m.to, m.ForData())
	}
	if err != nil {
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
