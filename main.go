package main

import (
	"flag"
	"fmt"
	"github.com/MikkelHJuul/smtp-client-simple/handler"
	"log"
	"net/http"
	"os"
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
	defaultValues := map[string]string{
		"to":      *defTo,
		"from":    *defFrom,
		"subject": *defSubj,
		"body":    *defMsg,
	}
	log.Print("Initiating Handler")
	h := handler.SmtpHandler(*smtpAddr, *reqFrom, *skipTls, defaultValues)
	log.Printf("\t%s: %s", "smtp address", *smtpAddr)
	log.Printf("\t%s: %s", "locked mail-from", *reqFrom)
	log.Printf("\t%s: ", "mail-defaults")
	for k, v := range defaultValues {
		log.Printf("\t\t%s: %s", k, v)
	}
	if *skipTls {
		log.Printf("%s", "WARN: this instance is running without tls")
	}

	log.Printf("Smtp server listening on port %d.\n", *port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), h); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
