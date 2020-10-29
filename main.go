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
	smtpAddr = flag.String("smtp_server", "", "Default server:port for smtp communications.")
	defFrom  = flag.String("from", "", "Default sender address.")
	defTo    = flag.String("to", "", "Default recipient address.")
	defSubj  = flag.String("subject", "", "Default message subject.")
	defMsg   = flag.String("message", "", "Default mail text.")
	reqFrom  = flag.String("forced_from", "", "If set, this is the sender, regardless of request parameters.")
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

type mail struct {
	from, to, subject, body string
}

func (m *mail) String() string {
	return fmt.Sprintf("From: %s\nTo: %s\nSubject: %s\n\n%s\n", m.from, m.to, m.subject, m.body)
}

type SmtpHandler struct {
	smtpAddr   string
	lockedFrom string
	defaults   map[string]string
}

func (smtpH *SmtpHandler) reqFieldOrDefault(req *http.Request, field string) string {
	if f := req.FormValue(field); f != "" {
		return f
	}
	return smtpH.defaults[field]
}

func (smtpH *SmtpHandler) newMailFromRequest(req *http.Request) (*mail, error) {
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
		to:      smtpH.reqFieldOrDefault(req, "to"),
		from:    smtpH.reqFieldOrDefault(req, "from"),
		subject: smtpH.reqFieldOrDefault(req, "subject"),
		body:    smtpH.reqFieldOrDefault(req, "msg"),
	}

	if m.body == "" && body != "" {
		log.Print("Using mail body from POST.")
		m.body = body
	}

	if smtpH.lockedFrom != "" {
		m.from = smtpH.lockedFrom
	}

	if m.to == "" || m.from == "" || m.subject == "" || m.body == "" {
		return nil, errors.New("Missing field in mail. Set appropriate query params (to, from, subject) and either set msg as query param or send POST body.")
	}

	return m, nil
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

	wr.Write([]byte(m.String()))
}

func (smtpH *SmtpHandler) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	log.Print(req.RemoteAddr + " | " + req.Method + " " + req.URL.String())

	m, err := smtpH.newMailFromRequest(req)
	if err != nil {
		respondError(wr, err)
	}

	if err := smtpH.sendMail(m); err != nil {
		respondError(wr, err)
	} else {
		respondOk(wr, m)
	}
}

func (smtpH *SmtpHandler) sendMail(m *mail) error {
	log.Printf("Calling %s", smtpH.smtpAddr)
	c, err := smtp.Dial(smtpH.smtpAddr)
	if err != nil {
		return fmt.Errorf("Error dialing smtp server: %s", err)
	}

	if err := c.Mail(m.from); err != nil {
		return fmt.Errorf("Error sending MAIL FROM: %s", err)
	}

	for _, to := range strings.Split(m.to, ",") {
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("Error sending RCPT TO: %s", err)
		}
	}

	// Send the email body.
	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("Error sending DATA command: %s", err)
	}
	if _, err = wc.Write([]byte(m.String())); err != nil {
		return fmt.Errorf("Error writing DATA: %s", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("Error closing connection: %s", err)
	}

	return c.Quit()
}
