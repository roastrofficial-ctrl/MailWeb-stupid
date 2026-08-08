package main

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var formFieldName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,63}$`)

type MailWebRequest struct {
	MailWeb string            `json:"mailweb"`
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	URI     string            `json:"uri"`
	Headers map[string]string `json:"headers"`
	Body    map[string]any    `json:"body,omitempty"`
}

type MailWebResponse struct {
	MailWeb   string           `json:"mailweb"`
	RequestID string           `json:"request_id"`
	Status    int              `json:"status"`
	Document  *MailWebDocument `json:"document"`
}

type MailWebDocument struct {
	Title string `json:"title"`
	Body  []Node `json:"body"`
}

type Node struct {
	Type   string      `json:"type"`
	Level  int         `json:"level,omitempty"`
	Text   string      `json:"text,omitempty"`
	Label  string      `json:"label,omitempty"`
	Href   string      `json:"href,omitempty"`
	Src    string      `json:"src,omitempty"`
	Alt    string      `json:"alt,omitempty"`
	Method string      `json:"method,omitempty"`
	Action string      `json:"action,omitempty"`
	Fields []FormField `json:"fields,omitempty"`
	Submit string      `json:"submit,omitempty"`
}

type FormField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

func NewRequest(uri string) (MailWebRequest, error) {
	return NewRequestWithBody("GET", uri, nil)
}

func NewRequestWithBody(method, uri string, body map[string]any) (MailWebRequest, error) {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "mailweb" || parsed.Host == "" {
		return MailWebRequest{}, errors.New("URI must be absolute and use mailweb://")
	}
	method = strings.ToUpper(method)
	if method != "GET" && method != "POST" {
		return MailWebRequest{}, errors.New("MailWeb 0.2 supports GET and POST only")
	}
	if method == "GET" && body != nil {
		return MailWebRequest{}, errors.New("GET requests must not contain a body")
	}
	headers := map[string]string{}
	if method == "POST" {
		headers["content-type"] = "application/json"
		if body == nil {
			body = map[string]any{}
		}
	}
	return MailWebRequest{
		MailWeb: "0.2",
		ID:      newID(),
		Method:  method,
		URI:     parsed.String(),
		Headers: headers,
		Body:    body,
	}, nil
}

func ValidateResponse(request MailWebRequest, response MailWebResponse) error {
	if response.MailWeb != request.MailWeb || (response.MailWeb != "0.1" && response.MailWeb != "0.2") {
		return fmt.Errorf("unsupported MailWeb version %q", response.MailWeb)
	}
	if response.RequestID != request.ID {
		return errors.New("response request_id does not match the request id")
	}
	if response.Status < 100 || response.Status > 599 {
		return fmt.Errorf("invalid MailWeb status %d", response.Status)
	}
	if response.Document == nil {
		return errors.New("response document is required")
	}
	if len(response.Document.Title) > 512 || len(response.Document.Body) > 10000 {
		return errors.New("document exceeds protocol limits")
	}
	for index, node := range response.Document.Body {
		if response.MailWeb == "0.1" && node.Type == "form" {
			return fmt.Errorf("document body node %d: form requires MailWeb 0.2", index)
		}
		if err := validateNode(node); err != nil {
			return fmt.Errorf("document body node %d: %w", index, err)
		}
	}
	return nil
}

func validateNode(node Node) error {
	switch node.Type {
	case "heading":
		if node.Level < 1 || node.Level > 6 {
			return errors.New("heading level must be between 1 and 6")
		}
		if node.Label != "" || node.Href != "" || node.Src != "" || node.Alt != "" || hasFormMetadata(node) {
			return errors.New("heading contains fields from another node type")
		}
	case "paragraph":
		if node.Level != 0 || node.Label != "" || node.Href != "" || node.Src != "" || node.Alt != "" || hasFormMetadata(node) {
			return errors.New("paragraph contains fields from another node type")
		}
	case "link", "button":
		if node.Label == "" || !safeReference(node.Href) {
			return errors.New("navigation node requires a label and safe href")
		}
		if node.Level != 0 || node.Text != "" || node.Src != "" || node.Alt != "" || hasFormMetadata(node) {
			return errors.New("navigation node contains fields from another node type")
		}
	case "image":
		if !safeReference(node.Src) {
			return errors.New("image requires a safe src")
		}
		if node.Level != 0 || node.Text != "" || node.Label != "" || node.Href != "" || hasFormMetadata(node) {
			return errors.New("image contains fields from another node type")
		}
	case "form":
		if node.Method != "GET" && node.Method != "POST" {
			return errors.New("form method must be GET or POST")
		}
		if !safeMailWebReference(node.Action) || node.Submit == "" || len(node.Fields) == 0 || len(node.Fields) > 100 {
			return errors.New("form requires a safe action, submit label, and 1 to 100 fields")
		}
		if node.Level != 0 || node.Text != "" || node.Label != "" || node.Href != "" || node.Src != "" || node.Alt != "" {
			return errors.New("form contains fields from another node type")
		}
		seen := map[string]bool{}
		for _, field := range node.Fields {
			if !formFieldName.MatchString(field.Name) || field.Type != "text" || field.Label == "" || seen[field.Name] {
				return errors.New("form fields require a unique name, text type, and label")
			}
			seen[field.Name] = true
		}
	default:
		return fmt.Errorf("unsupported type %q", node.Type)
	}
	return nil
}

func hasFormMetadata(node Node) bool {
	return node.Method != "" || node.Action != "" || len(node.Fields) != 0 || node.Submit != ""
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

func safeMailWebReference(value string) bool {
	if !safeReference(value) {
		return false
	}
	reference, _ := url.Parse(value)
	return reference.Scheme == "" || strings.EqualFold(reference.Scheme, "mailweb")
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
