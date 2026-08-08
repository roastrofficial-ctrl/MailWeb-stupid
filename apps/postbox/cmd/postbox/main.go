package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type requestMessage struct {
	MailWeb string            `json:"mailweb"`
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	URI     string            `json:"uri"`
	Headers map[string]string `json:"headers"`
}

type responseMessage struct {
	MailWeb   string   `json:"mailweb"`
	RequestID string   `json:"request_id"`
	Status    int      `json:"status"`
	Document  document `json:"document"`
}

type document struct {
	Title string `json:"title"`
	Body  []node `json:"body"`
}

type node struct {
	Type  string `json:"type"`
	Level int    `json:"level,omitempty"`
	Text  string `json:"text,omitempty"`
	Label string `json:"label,omitempty"`
	Href  string `json:"href,omitempty"`
	Src   string `json:"src,omitempty"`
	Alt   string `json:"alt,omitempty"`
}

type server struct {
	publisherURL string
	client       *http.Client
}

func main() {
	port := envOr("PORT", "8080")
	publisherURL := envOr("MAILWEB_PUBLISHER_URL", "http://localhost:8081/mailweb/messages")
	s := &server{publisherURL: publisherURL, client: &http.Client{Timeout: 5 * time.Second}}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /", s.render)

	log.Printf("postbox listening on :%s; publisher=%s", port, publisherURL)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func (s *server) render(w http.ResponseWriter, r *http.Request) {
	message := requestMessage{
		MailWeb: "0.1",
		ID:      newID(),
		Method:  r.Method,
		URI:     "mailweb://demo.local" + r.URL.RequestURI(),
		Headers: map[string]string{},
	}

	response, err := s.exchange(r.Context(), message)
	if err != nil {
		log.Printf("message exchange failed: %v", err)
		http.Error(w, "MailWeb publisher unavailable", http.StatusBadGateway)
		return
	}

	body, err := renderDocument(response.Document)
	if err != nil {
		log.Printf("document rendering failed: %v", err)
		http.Error(w, "MailWeb document invalid", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-MailWeb-Request-ID", response.RequestID)
	w.WriteHeader(response.Status)
	_, _ = w.Write(body)
}

func (s *server) exchange(ctx context.Context, message requestMessage) (responseMessage, error) {
	payload, err := json.Marshal(message)
	if err != nil {
		return responseMessage{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.publisherURL, bytes.NewReader(payload))
	if err != nil {
		return responseMessage{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := s.client.Do(req)
	if err != nil {
		return responseMessage{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return responseMessage{}, fmt.Errorf("publisher returned HTTP %d", res.StatusCode)
	}

	var reply responseMessage
	decoder := json.NewDecoder(io.LimitReader(res.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&reply); err != nil {
		return responseMessage{}, fmt.Errorf("decode response: %w", err)
	}
	if reply.MailWeb != message.MailWeb || reply.RequestID != message.ID {
		return responseMessage{}, errors.New("publisher returned an invalid response envelope")
	}
	if reply.Status < 100 || reply.Status > 599 {
		return responseMessage{}, errors.New("publisher returned an invalid document status")
	}
	return reply, nil
}

var pageTemplate = template.Must(template.New("document").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}}</title><style>body{font:1rem/1.6 system-ui,sans-serif;max-width:44rem;margin:10vh auto;padding:0 1.5rem}img{max-width:100%}.button{display:inline-block;padding:.6rem 1rem;border:1px solid;border-radius:.5rem;text-decoration:none}</style></head>
<body><main>{{range .Nodes}}{{.}}{{end}}</main></body></html>`))

func renderDocument(doc document) ([]byte, error) {
	var nodes []template.HTML
	for _, item := range doc.Body {
		var rendered string
		switch item.Type {
		case "heading":
			if item.Level < 1 || item.Level > 6 {
				return nil, errors.New("invalid heading level")
			}
			rendered = fmt.Sprintf("<h%d>%s</h%d>", item.Level, template.HTMLEscapeString(item.Text), item.Level)
		case "paragraph":
			rendered = "<p>" + template.HTMLEscapeString(item.Text) + "</p>"
		case "link", "button":
			if !safeReference(item.Href) {
				return nil, errors.New("unsafe navigation URI")
			}
			class := ""
			if item.Type == "button" {
				class = ` class="button"`
			}
			rendered = `<p><a` + class + ` href="` + template.HTMLEscapeString(item.Href) + `">` + template.HTMLEscapeString(item.Label) + `</a></p>`
		case "image":
			if !safeReference(item.Src) {
				return nil, errors.New("unsafe image URI")
			}
			rendered = `<img src="` + template.HTMLEscapeString(item.Src) + `" alt="` + template.HTMLEscapeString(item.Alt) + `">`
		default:
			return nil, fmt.Errorf("unsupported node type %q", item.Type)
		}
		nodes = append(nodes, template.HTML(rendered))
	}
	var output bytes.Buffer
	err := pageTemplate.Execute(&output, struct {
		Title string
		Nodes []template.HTML
	}{doc.Title, nodes})
	return output.Bytes(), err
}

func safeReference(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	reference, err := url.Parse(value)
	if err != nil {
		return false
	}
	switch strings.ToLower(reference.Scheme) {
	case "", "mailweb", "http", "https":
		return true
	default:
		return false
	}
}

func newID() string {
	var value [16]byte
	milliseconds := uint64(time.Now().UnixMilli())
	value[0] = byte(milliseconds >> 40)
	value[1] = byte(milliseconds >> 32)
	value[2] = byte(milliseconds >> 24)
	value[3] = byte(milliseconds >> 16)
	value[4] = byte(milliseconds >> 8)
	value[5] = byte(milliseconds)
	if _, err := rand.Read(value[6:]); err != nil {
		panic(err)
	}
	encoding := base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)
	return encoding.EncodeToString(value[:])
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
