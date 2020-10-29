package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
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
	fmt.Printf("Initiating Handler")
	handler := SmtpHandler{
		smtpAddr:       *smtpAddr,
		defaultFrom:    *defFrom,
		defaultTo:      *defTo,
		defaultSubject: *defSubj,
		defaultBody:    *defMsg,
		lockFrom:       *reqFrom,
	}

	fmt.Printf("Smtp server listening on port %d.\n", *port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), &handler)
	if err != nil {
		panic(err)
	}
}

func valueOrDefault(val string, deflt string) string {
	if val == "" {
		return deflt
	}
	return val
}

type SmtpHandler struct {
	smtpAddr, defaultFrom, defaultTo, defaultSubject, defaultBody, lockFrom string
}

func (smtpH *SmtpHandler) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	fmt.Println(req.RemoteAddr + " | " + req.Method + " " + req.URL.String())
	if err := smtpH.sendMail(req); err != nil {
		serverError(wr, err)
	} else {
		serveHTTP(wr, req)
	}
}

func serverError(wr http.ResponseWriter, err error) {
	wr.Header().Add("Content-Type", "text/plain")
	wr.WriteHeader(400)
	_, _ = fmt.Fprintf(wr, err.Error())
}

func (smtpH *SmtpHandler) getFromMail(requestFrom string) string {
	if smtpH.lockFrom != "" {
		return smtpH.lockFrom
	}
	if requestFrom != "" {
		return requestFrom
	}
	return smtpH.defaultFrom
}

func printIf(err error) {
	if err != nil {
		log.Print(err)
	}
}

func (smtpH *SmtpHandler) sendMail(req *http.Request) error {
	// Connect to the remote SMTP server.
	c, err := smtp.Dial(smtpH.smtpAddr)
	printIf(err)

	if fromMail := smtpH.getFromMail(req.FormValue("from")); fromMail == "" {
		return errors.New("you have to set the query-parameter 'from', or this server-instance is mis-configured")
	} else {
		// Set the sender and recipient first
		err = c.Mail(fromMail)
		printIf(err)
	}
	toMail, ok := req.Form["to"]
	if !ok {
		if smtpH.defaultTo != "" {
			err = c.Rcpt(smtpH.defaultTo)
			printIf(err)
		} else {
			return errors.New("you have to set the query-parameter 'to'")
		}
	}
	for _, mail := range toMail {
		err = c.Rcpt(mail)
		printIf(err)
	}

	// Send the email body.
	wc, err := c.Data()
	printIf(err)
	emailMsg, err := ioutil.ReadAll(req.Body)
	printIf(err)
	msg := smtpH.MailMessage(string(emailMsg), req.FormValue("subject"), req.FormValue("msg"))
	_, err = wc.Write([]byte(msg))
	printIf(err)

	err = wc.Close()
	printIf(err)

	// Send the QUIT command and close the connection.
	err = c.Quit()
	printIf(err)
	return nil
}

func (smtpH *SmtpHandler) MailMessage(msgFromBody string, subjFromQuery string, msgFromQuery string) string {
	if msgFromBody != "" {
		if strings.Contains(msgFromBody, "Subject: ") {
			return msgFromBody //if anyone ever writes 'Subject: ' somewhere in the mail... this happens
		}
		if subject := valueOrDefault(subjFromQuery, smtpH.defaultSubject); subject != "" {
			return "Subject: " + subject + "\n\n" + msgFromBody
		}
		return msgFromBody
	}
	msg := valueOrDefault(msgFromQuery, smtpH.defaultBody)
	if subject := valueOrDefault(subjFromQuery, smtpH.defaultSubject); subject != "" {
		return "Subject: " + subject + "\n\n" + msg
	}
	return msg
}

func serveHTTP(wr http.ResponseWriter, req *http.Request) {
	wr.Header().Add("Content-Type", "text/plain")
	wr.WriteHeader(200)

	_, _ = fmt.Fprintf(wr, "%s %s %s\n", req.Proto, req.Method, req.URL)
	_, _ = fmt.Fprintf(wr, "Host: %s\n", req.Host)
	for key, values := range req.Form {
		for _, value := range values {
			_, _ = fmt.Fprintf(wr, "%s: %s\n", key, value)
		}
	}
}
