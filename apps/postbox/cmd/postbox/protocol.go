package main

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type MailWebRequest struct {
	MailWeb string            `json:"mailweb"`
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	URI     string            `json:"uri"`
	Headers map[string]string `json:"headers"`
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
	Type  string `json:"type"`
	Level int    `json:"level,omitempty"`
	Text  string `json:"text,omitempty"`
	Label string `json:"label,omitempty"`
	Href  string `json:"href,omitempty"`
	Src   string `json:"src,omitempty"`
	Alt   string `json:"alt,omitempty"`
}

func NewRequest(uri string) (MailWebRequest, error) {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "mailweb" || parsed.Host == "" {
		return MailWebRequest{}, errors.New("URI must be absolute and use mailweb://")
	}
	return MailWebRequest{
		MailWeb: "0.1",
		ID:      newID(),
		Method:  "GET",
		URI:     parsed.String(),
		Headers: map[string]string{},
	}, nil
}

func ValidateResponse(request MailWebRequest, response MailWebResponse) error {
	if response.MailWeb != "0.1" {
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
		if node.Label != "" || node.Href != "" || node.Src != "" || node.Alt != "" {
			return errors.New("heading contains fields from another node type")
		}
	case "paragraph":
		if node.Level != 0 || node.Label != "" || node.Href != "" || node.Src != "" || node.Alt != "" {
			return errors.New("paragraph contains fields from another node type")
		}
	case "link", "button":
		if node.Label == "" || !safeReference(node.Href) {
			return errors.New("navigation node requires a label and safe href")
		}
		if node.Level != 0 || node.Text != "" || node.Src != "" || node.Alt != "" {
			return errors.New("navigation node contains fields from another node type")
		}
	case "image":
		if !safeReference(node.Src) {
			return errors.New("image requires a safe src")
		}
		if node.Level != 0 || node.Text != "" || node.Label != "" || node.Href != "" {
			return errors.New("image contains fields from another node type")
		}
	default:
		return fmt.Errorf("unsupported type %q", node.Type)
	}
	return nil
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
