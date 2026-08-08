package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Transport carries MailWeb messages. Its implementation is not part of the
// MailWeb protocol, so SMTP or file transports can be added without changing
// MailWebRequest or MailWebResponse.
type Transport interface {
	Exchange(context.Context, MailWebRequest) (MailWebResponse, error)
}

type HTTPTransport struct {
	PublisherURL string
	Client       *http.Client
}

func NewHTTPTransport(publisherURL string) *HTTPTransport {
	return &HTTPTransport{
		PublisherURL: publisherURL,
		Client:       &http.Client{Timeout: 5 * time.Second},
	}
}

func (transport *HTTPTransport) Exchange(ctx context.Context, message MailWebRequest) (MailWebResponse, error) {
	payload, err := json.Marshal(message)
	if err != nil {
		return MailWebResponse{}, err
	}
	observeJourney(ctx, "request.serialized", "MailWebRequest serialized as application/json", map[string]string{"bytes": jsonNumber(len(payload))})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, transport.PublisherURL, bytes.NewReader(payload))
	if err != nil {
		return MailWebResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	observeJourney(ctx, "transport.http.started", "direct carrier exchange started", map[string]string{"endpoint": transport.PublisherURL, "request_id": message.ID})

	response, err := transport.Client.Do(request)
	if err != nil {
		return MailWebResponse{}, err
	}
	observeJourney(ctx, "transport.http.response", "direct carrier response received", map[string]string{"status": response.Status})
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return MailWebResponse{}, fmt.Errorf("local publisher returned carrier status %d: %s", response.StatusCode, body)
	}

	var messageResponse MailWebResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&messageResponse); err != nil {
		return MailWebResponse{}, fmt.Errorf("decode MailWebResponse: %w", err)
	}
	return messageResponse, nil
}
