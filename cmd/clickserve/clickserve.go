package main

import (
	"encoding/json"
	"fmt"
	flag "github.com/spf13/pflag"
	"github.com/wttw/clickcheck"
	"html"
	"log"
	"net/http"
	"os"
	"strings"
)

const appName = "clickserve"

func main() {
	var configFile string
	helpRequest := false
	flag.StringVar(&configFile, "config", "clickcheck.yaml", "Alternate configuration file")
	flag.BoolVarP(&helpRequest, "help", "h", false, "Display brief help")

	flag.Parse()
	if helpRequest {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", appName)
		flag.PrintDefaults()
		os.Exit(0)
	}
	// Get our configuration
	c := clickcheck.New(configFile)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//addr, port, err := net.SplitHostPort(r.RemoteAddr)
		//if err != nil {
		//	log.Printf("Bad remoteaddr: %s", r.RemoteAddr)
		//}

		if !strings.HasPrefix(r.Host, "click") {
			http.Error(w, "piss off", 404)
			return
		}

		err := r.ParseForm()
		if err != nil {
			log.Printf("Failed to parse values for '%s': %s", r.URL.String(), err)
		}
		log.Printf("Request for %s %s from %s", r.Host, r.URL, r.RemoteAddr)
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		encoder.Encode(r.Form)
		fmt.Fprintf(w, "Hello from %q", html.EscapeString(r.URL.Path))
	})
	log.Printf("Listening on %s", c.Listen)
	var err error
	if c.Cert != "" {
		err = http.ListenAndServeTLS(c.Listen, c.Cert, c.Key, handler)
	} else {
		err = http.ListenAndServe(c.Listen, handler)
	}
	if err != nil {
		log.Fatalf("Server exiting: %s", err)
	}
}
