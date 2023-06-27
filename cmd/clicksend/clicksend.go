package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/wttw/clickcheck"
	"log"
	"net"
	"net/smtp"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	flag "github.com/spf13/pflag"
)

const appName = "clicksend"

const alphabet = "abcdefghijklmnopqrstuvwxyz"

func main() {
	var configFile string
	var recipient string
	var from string
	var tplName string
	var note string
	var path string
	parameters := url.Values{}
	helpRequest := false

	flag.StringVar(&configFile, "config", "clickcheck.yaml", "Alternate configuration file")
	flag.StringVar(&recipient, "to", "", "Recipient")
	flag.StringVar(&from, "from", "", "822.From")
	flag.StringVar(&tplName, "template", "default", "Template for message")
	flag.StringVar(&note, "note", "", "Mark the mail sent with this note")
	flag.BoolVarP(&helpRequest, "help", "h", false, "Display brief help")
	flag.StringVar(&path, "path", "", "Path for link")

	flag.Parse()
	if helpRequest {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", appName)
		flag.PrintDefaults()
		os.Exit(0)
	}

	c := clickcheck.New(configFile)

	// Use some defaults from our config file
	if recipient == "" {
		recipient = c.To
	}
	if from == "" {
		from = c.From
	}

	var args []string

	if len(flag.Args()) == 1 {
		args = strings.Split(flag.Args()[0], "&")
	} else {
		args = flag.Args()
	}

	for _, v := range args {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 1 {
			parts = append(parts, "")
		}
		parameters.Add(parts[0], parts[1])
	}

	u, err := url.Parse(c.URL)
	if err != nil {
		log.Fatalf("failed to parse %s: %s", c.URL, err)
	}
	if path != "" {
		u.Path = path
	}
	u.RawQuery = parameters.Encode()

	link := u.String()

	// And we need a message template
	tplPath := filepath.Join(c.Templates, strings.TrimSuffix(tplName, ".tpl")+".tpl")
	emailTpl, err := template.ParseFiles(tplPath)
	if err != nil {
		log.Fatalf("Failed to open message template: %s", err)
	}

	var payload bytes.Buffer
	err = emailTpl.Execute(&payload, struct {
		Date string
		To   string
		From string
		Note string
		Link string
	}{
		Date: time.Now().Format(time.RFC822Z),
		To:   recipient,
		From: from,
		Note: note,
		Link: link,
	})
	if err != nil {
		log.Fatalf("Failed to build message: %s", err)
	}

	// Convert to CRLF line endings, because we've read RFC 821
	var enc []byte
	for _, c := range payload.Bytes() {
		switch c {
		case '\n':
			enc = append(enc, '\r', '\n')
		case '\r':
		default:
			enc = append(enc, c)
		}
	}
	host, _, err := net.SplitHostPort(c.Smarthost)
	if err != nil {
		log.Fatalf("Malformed smarthost string: %s", err)
	}

	// fmt.Printf("Sending mail...\n%s\n", string(enc))
	auth := smtp.PlainAuth("", c.Username, c.Password, host)
	err = smtp.SendMail(c.Smarthost, auth, from, []string{recipient}, enc)
	if err != nil {
		log.Fatalf("Failed to send email: %s", err)
	}
	fmt.Printf("Mail sent to %s\n%s\n", recipient, link)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	encoder.Encode(parameters)
}

func newImage(c clickcheck.Config, db *pgxpool.Pool, imageFile string) (string, int) {
	// Create a new tracking image
	urlTemplate, err := template.New("path").Parse(c.URL)
	if err != nil {
		log.Fatalf("Failed to parse urlString template: %s", err)
	}
	if _, err := os.Stat(filepath.Join(c.ImageDir, imageFile)); os.IsNotExist(err) {
		log.Fatalf("Image file %s doesn't exist", imageFile)
	}
	var imageID int
	// language=SQL
	err = db.QueryRow(context.Background(), `insert into image (file) values ($1) returning id`, imageFile).Scan(&imageID)
	if err != nil {
		log.Fatalf("Failed to create image: %s", err)
	}

	// let's make some base26
	encoded := ""
	n := imageID
	for n > 0 {
		remainder := n % len(alphabet)
		encoded = alphabet[remainder:remainder+1] + encoded
		n /= len(alphabet)
	}
	var urlString bytes.Buffer
	err = urlTemplate.Execute(&urlString,
		struct {
			ImageID     int
			ImageCookie string
			ImageFile   string
		}{
			ImageID:     imageID,
			ImageCookie: encoded,
			ImageFile:   imageFile,
		})
	if err != nil {
		log.Fatalf("Failed to execute url template: %s", err)
	}
	u, err := url.Parse(urlString.String())
	if err != nil {
		log.Fatalf("Generated an invalid URL: %s", err)
	}
	imageURL := u.String()
	// language=SQL
	_, err = db.Exec(context.Background(), `update image set url=$1, path=$2 where id=$3`, imageURL, u.Path, imageID)
	if err != nil {
		log.Fatalf("Failed to update image: %s", err)
	}
	return imageURL, imageID
}
