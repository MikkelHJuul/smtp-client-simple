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
	}
	
	fmt.Printf("Smtp server listening on port %s.\n", port)
	err := http.ListenAndServe(":"+port, &handler)
	if err != nil {
		panic(err)
	}
}

//https://stackoverflow.com/questions/56616196/how-to-convert-camel-case-string-to-snake-case
var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")

func toScreamingSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake  = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
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
	smtpAddr, defaultFrom, defaultTo, defaultSubject, defaultBody string
}

func (smtpH* SmtpHandler) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
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

func (smtpH* SmtpHandler) sendMail(req *http.Request) error {
	// Connect to the remote SMTP server.
	c, err := smtp.Dial(smtpH.smtpAddr)
	if err != nil {
		log.Print(err)
	}

	if fromMail := valueOrDefault(req.FormValue("from"), smtpH.defaultFrom); fromMail == "" {
		return errors.New("you have to set the query-parameter 'from'")
	} else {
		// Set the sender and recipient first
		if err := c.Mail(fromMail); err != nil {
			log.Print(err)
		}
	}
	if toMail, ok := req.Form["to"]; !ok {
		if smtpH.defaultTo != "" {
			if err := c.Rcpt(smtpH.defaultTo); err != nil {
				log.Print(err)
			}
		} else {
			return errors.New("you have to set the query-parameter 'to'")
		}
	} else {
		for _, mail := range toMail {
			if err := c.Rcpt(mail); err != nil {
				log.Print(err)
			}
		}
	}

	// Send the email body.
	wc, _ := c.Data()
	if err != nil {
		log.Print(err)
	}
	if subject := valueOrDefault(req.FormValue("subject"), smtpH.defaultSubject); subject != "" {
		_, err = fmt.Fprintf(wc, "Subject: " + subject)
	}
	if err != nil {
		log.Print(err)
	}
	if emailMsg, err := ioutil.ReadAll(req.Body); err != nil {
		log.Print(err)
	} else {
		if body := valueOrDefault(string(emailMsg), valueOrDefault(req.FormValue("msg"), smtpH.defaultBody)); body != "" {
			_, err = fmt.Fprintf(wc, body)
			if err != nil {
				log.Print(err)
			}
		}
	}
	err = wc.Close()
	if err != nil {
		log.Print(err)
	}

	// Send the QUIT command and close the connection.
	err = c.Quit()
	if err != nil {
		log.Print(err)
	}
	return nil
}


func serveHTTP(wr http.ResponseWriter, req *http.Request) {
	wr.Header().Add("Content-Type", "text/plain")
	wr.WriteHeader(200)

	_, _ = fmt.Fprintf(wr, "%s %s %s\n", req.Proto, req.Method, req.URL)
	_, _ = fmt.Fprintln(wr, "")
	_, _ = fmt.Fprintf(wr, "Host: %s\n", req.Host)
	for key, values := range req.Form {
		for _, value := range values {
			_, _ = fmt.Fprintf(wr, "%s: %s\n", key, value)
		}
	}
}
