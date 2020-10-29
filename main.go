package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"regexp"
	"strings"
)

func main() {
	port := valueFromENVorDefault("port", "8080")
	fmt.Printf("Initiating Handler")
	handler := SmtpHandler{
		smtpAddr:       valueFromENVorDefault("smtpAddr", ""),
		defaultFrom:    valueFromENVorDefault("defaultFrom", ""),
		defaultTo:      valueFromENVorDefault("defaultTo", ""),
		defaultSubject: valueFromENVorDefault("defaultSubject", ""),
		defaultBody:    valueFromENVorDefault("defaultBody", ""),
		lockFrom:       valueFromENVorDefault("lockFrom", ""),
	}

	fmt.Printf("Smtp server listening on port %s.\n", port)
	err := http.ListenAndServe(":"+port, &handler)
	if err != nil {
		panic(err)
	}
}

//https://stackoverflow.com/questions/56616196/how-to-convert-camel-case-string-to-snake-case
var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func toScreamingSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToUpper(snake)
}

func valueFromENVorDefault(name string, deflt string) string {
	return valueOrDefault(os.Getenv(toScreamingSnakeCase(name)), deflt)
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
