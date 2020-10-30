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
		if strings.Contains(body, "From: ") {
			m.from = ""
		}
		if strings.Contains(body, "To: ") {
			m.to = []string{""}
		}
	}

	if s.lockedFrom != "" {
		m.from = s.lockedFrom
		//From/To in the mail body may be set differently - how does that work?
	}

	if len(m.to) == 0 || m.from == "" {
		return nil, errors.New("missing fields in mail. Set appropriate parameters: to, from etc.")
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

	if err := smtp.SendMail(s.smtpAddr, nil, m.from, m.to, m.ForData()); err != nil {
		respondError(wr, err)
	} else {
		respondOk(wr, m)
	}
}
