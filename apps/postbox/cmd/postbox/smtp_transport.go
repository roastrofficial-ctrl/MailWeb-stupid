package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"time"
)

type SMTPTransport struct {
	SMTPAddress   string
	Publisher     string
	MailpitURL    string
	Timeout       time.Duration
	PollEvery     time.Duration
	Client        *http.Client
	Status        io.Writer
	ClientMailbox string
	Publishers map[string]string
}

func NewSMTPTransport(smtpAddress, publisher, mailpitURL string, timeout time.Duration, status io.Writer) *SMTPTransport {
	return &SMTPTransport{
		SMTPAddress:   smtpAddress,
		Publisher:     publisher,
		MailpitURL:    strings.TrimRight(mailpitURL, "/"),
		Timeout:       timeout,
		PollEvery:     500 * time.Millisecond,
		Client:        &http.Client{Timeout: 3 * time.Second},
		Status:        status,
		ClientMailbox: "postbox-" + strings.ToLower(newID()) + "@client.local", Publishers: publisherMap(),
	}
}

func (transport *SMTPTransport) Mailbox() string {
	return transport.ClientMailbox
}

func (transport *SMTPTransport) Correspondent() string { return transport.Publisher }

func (transport *SMTPTransport) CorrespondentFor(rawURI string) string {
	parsed, err := url.Parse(rawURI)
	if err == nil {
		if mapped := transport.Publishers[parsed.Host]; mapped != "" {
			return mapped
		}
	}
	return transport.Publisher
}

func (transport *SMTPTransport) Exchange(ctx context.Context, request MailWebRequest) (MailWebResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return MailWebResponse{}, err
	}
	observeJourney(ctx, "request.serialized", "MailWebRequest serialized as application/mailweb+json", map[string]string{"bytes": jsonNumber(len(payload))})
	mailbox := transport.ClientMailbox
	destination := transport.CorrespondentFor(request.URI)
	message := buildMailMessage(mailbox, destination, "MailWeb request "+request.ID, payload)
	if err := smtp.SendMail(transport.SMTPAddress, nil, mailbox, []string{destination}, message); err != nil {
		return MailWebResponse{}, fmt.Errorf("send request through SMTP: %w", err)
	}
	observeJourney(ctx, "transport.smtp.sent", "SMTP correspondence accepted for delivery", map[string]string{"from": mailbox, "to": destination, "request_id": request.ID})
	if transport.Status != nil {
		fmt.Fprintln(transport.Status, "Waiting for the publisher to reply...")
	}

	waitCtx, cancel := context.WithTimeout(ctx, transport.Timeout)
	defer cancel()
	ticker := time.NewTicker(transport.PollEvery)
	observeJourney(ctx, "mailbox.polling", "polling private mailbox for correlated reply", map[string]string{"mailbox": mailbox})
	defer ticker.Stop()
	for {
		response, found, err := transport.findResponse(waitCtx, mailbox, request.ID)
		if err != nil {
			if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
				return MailWebResponse{}, fmt.Errorf("timed out after %s waiting for a reply", transport.Timeout)
			}
			return MailWebResponse{}, err
		}
		if found {
			observeJourney(ctx, "mailbox.response_detected", "correlated correspondence detected", map[string]string{"request_id": request.ID})
			return response, nil
		}
		select {
		case <-waitCtx.Done():
			if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
				return MailWebResponse{}, fmt.Errorf("timed out after %s waiting for a reply", transport.Timeout)
			}
			return MailWebResponse{}, waitCtx.Err()
		case <-ticker.C:
		}
	}
}

func publisherMap() map[string]string { result := map[string]string{}; for _, pair := range strings.Split(envOr("MAILWEB_PUBLISHER_MAP", ""), ",") { parts := strings.SplitN(pair, "=", 2); if len(parts) == 2 { result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1]) } }; return result }

type mailpitMessages struct {
	Messages []struct {
		ID      string `json:"ID"`
		Subject string `json:"Subject"`
		To      []struct {
			Address string `json:"Address"`
		} `json:"To"`
	} `json:"messages"`
}

func (transport *SMTPTransport) findResponse(ctx context.Context, mailbox, requestID string) (MailWebResponse, bool, error) {
	query := url.Values{"query": {"to:" + mailbox}, "limit": {"20"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, transport.MailpitURL+"/api/v1/search?"+query.Encode(), nil)
	if err != nil {
		return MailWebResponse{}, false, err
	}
	response, err := transport.Client.Do(request)
	if err != nil {
		return MailWebResponse{}, false, fmt.Errorf("poll client mailbox: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return MailWebResponse{}, false, fmt.Errorf("Mailpit search returned %d", response.StatusCode)
	}
	var messages mailpitMessages
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&messages); err != nil {
		return MailWebResponse{}, false, fmt.Errorf("decode Mailpit search: %w", err)
	}
	for _, summary := range messages.Messages {
		if summary.Subject != "MailWeb response "+requestID {
			continue
		}
		raw, err := transport.rawMessage(ctx, summary.ID)
		if err != nil {
			return MailWebResponse{}, false, err
		}
		parsed, err := parseMailWebResponse(raw)
		if err != nil {
			return MailWebResponse{}, false, err
		}
		if parsed.RequestID == requestID {
			return parsed, true, nil
		}
	}
	return MailWebResponse{}, false, nil
}

func (transport *SMTPTransport) rawMessage(ctx context.Context, id string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, transport.MailpitURL+"/api/v1/message/"+url.PathEscape(id)+"/raw", nil)
	if err != nil {
		return nil, err
	}
	response, err := transport.Client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch Mailpit message: status %d", response.StatusCode)
	}
	return io.ReadAll(io.LimitReader(response.Body, 7<<20))
}

func buildMailMessage(from, to, subject string, payload []byte) []byte {
	var message strings.Builder
	fmt.Fprintf(&message, "From: %s\r\n", from)
	fmt.Fprintf(&message, "To: %s\r\n", to)
	fmt.Fprintf(&message, "Subject: %s\r\n", subject)
	fmt.Fprint(&message, "MIME-Version: 1.0\r\n")
	fmt.Fprint(&message, "Content-Type: application/mailweb+json; charset=utf-8\r\n")
	fmt.Fprint(&message, "Content-Transfer-Encoding: 8bit\r\n\r\n")
	message.Write(payload)
	return []byte(message.String())
}

func parseMailWebResponse(raw []byte) (MailWebResponse, error) {
	message, err := mail.ReadMessage(bufio.NewReader(strings.NewReader(string(raw))))
	if err != nil {
		return MailWebResponse{}, fmt.Errorf("parse response email: %w", err)
	}
	mediaType, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	if err != nil {
		return MailWebResponse{}, fmt.Errorf("response has content type %q, want application/mailweb+json", mediaType)
	}
	if strings.EqualFold(mediaType, "multipart/mixed") {
		return parseMultipartMailWebResponse(message, params["boundary"])
	}
	if !strings.EqualFold(mediaType, "application/mailweb+json") {
		return MailWebResponse{}, fmt.Errorf("response has content type %q, want application/mailweb+json", mediaType)
	}
	var body io.Reader = message.Body
	switch strings.ToLower(message.Header.Get("Content-Transfer-Encoding")) {
	case "base64":
		body = base64.NewDecoder(base64.StdEncoding, message.Body)
	case "quoted-printable":
		body = quotedprintable.NewReader(message.Body)
	case "", "7bit", "8bit", "binary":
	default:
		return MailWebResponse{}, errors.New("unsupported email transfer encoding")
	}
	var response MailWebResponse
	decoder := json.NewDecoder(io.LimitReader(body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return MailWebResponse{}, fmt.Errorf("decode application/mailweb+json response: %w", err)
	}
	return response, nil
}

func parseMultipartMailWebResponse(message *mail.Message, boundary string) (MailWebResponse, error) {
	if boundary == "" {
		return MailWebResponse{}, errors.New("multipart MailWeb response has no boundary")
	}
	reader := multipart.NewReader(message.Body, boundary)
	var response MailWebResponse
	foundJSON := false
	parts := map[string][]byte{}
	for count := 0; ; count++ {
		if count > 16 {
			return MailWebResponse{}, errors.New("too many MIME enclosure parts")
		}
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return MailWebResponse{}, fmt.Errorf("parse MIME enclosure: %w", err)
		}
		mediaType, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		body, err := readTransferDecoded(part, 3<<20)
		if err != nil {
			return MailWebResponse{}, err
		}
		if mediaType == "application/mailweb+json" {
			if foundJSON {
				return MailWebResponse{}, errors.New("duplicate MailWeb JSON MIME part")
			}
			if err := json.Unmarshal(body, &response); err != nil {
				return MailWebResponse{}, err
			}
			foundJSON = true
			continue
		}
		id := strings.Trim(part.Header.Get("Content-ID"), "<>")
		if id == "" || parts[id] != nil {
			return MailWebResponse{}, errors.New("invalid or duplicate enclosure Content-ID")
		}
		parts[id] = body
	}
	if !foundJSON {
		return MailWebResponse{}, errors.New("multipart response lacks MailWeb JSON part")
	}
	for index := range response.Enclosures {
		if body, ok := parts[response.Enclosures[index].ID]; ok {
			response.Enclosures[index].Content = body
			delete(parts, response.Enclosures[index].ID)
		}
	}
	if len(parts) != 0 {
		return MailWebResponse{}, errors.New("unreferenced MIME enclosure part")
	}
	return response, nil
}

func readTransferDecoded(part *multipart.Part, limit int64) ([]byte, error) {
	var reader io.Reader = part
	switch strings.ToLower(part.Header.Get("Content-Transfer-Encoding")) {
	case "base64":
		reader = base64.NewDecoder(base64.StdEncoding, part)
	case "quoted-printable":
		reader = quotedprintable.NewReader(part)
	}
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, errors.New("MIME enclosure exceeds size limit")
	}
	return body, nil
}
