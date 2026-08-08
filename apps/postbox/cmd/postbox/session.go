package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sync"
	"time"
)

type NavigationResult struct {
	URI                string          `json:"uri"`
	Request            MailWebRequest  `json:"request"`
	Response           MailWebResponse `json:"response"`
	Transport          string          `json:"transport"`
	RequestSentAt      time.Time       `json:"request_sent_at"`
	ResponseReceivedAt time.Time       `json:"response_received_at"`
	RoundTripMS        int64           `json:"round_trip_ms"`
	ClientMailbox      string          `json:"client_mailbox,omitempty"`
}

type BrowserState struct {
	Current           *NavigationResult `json:"current"`
	CanGoBack         bool              `json:"can_go_back"`
	CanGoForward      bool              `json:"can_go_forward"`
	SelectedTransport string            `json:"selected_transport"`
	ClientMailbox     string            `json:"client_mailbox,omitempty"`
}

// BrowserSession owns navigation semantics independently of terminal or web UI.
// A session serializes navigation so history and request correlation stay coherent.
type BrowserSession struct {
	mu            sync.Mutex
	transport     Transport
	transportName string
	clientMailbox string
	history       []NavigationResult
	index         int
}

func NewBrowserSession(transport Transport, transportName string) *BrowserSession {
	mailbox := ""
	if provider, ok := transport.(interface{ Mailbox() string }); ok {
		mailbox = provider.Mailbox()
	}
	return &BrowserSession{transport: transport, transportName: transportName, clientMailbox: mailbox, index: -1}
}

func (session *BrowserSession) Navigate(ctx context.Context, uri string) (BrowserState, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	result, err := session.exchange(ctx, uri)
	if err != nil {
		return session.state(), err
	}
	session.history = append(session.history[:session.index+1], result)
	session.index = len(session.history) - 1
	return session.state(), nil
}

func (session *BrowserSession) NavigateReference(ctx context.Context, reference string) (BrowserState, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.index < 0 {
		return session.state(), errors.New("cannot resolve a relative link without a current document")
	}
	uri, err := resolveMailWebReference(session.history[session.index].URI, reference)
	if err != nil {
		return session.state(), err
	}
	result, err := session.exchange(ctx, uri)
	if err != nil {
		return session.state(), err
	}
	session.history = append(session.history[:session.index+1], result)
	session.index = len(session.history) - 1
	return session.state(), nil
}

func (session *BrowserSession) Back(ctx context.Context) (BrowserState, error) {
	return session.move(ctx, -1)
}

func (session *BrowserSession) Forward(ctx context.Context) (BrowserState, error) {
	return session.move(ctx, 1)
}

func (session *BrowserSession) Reload(ctx context.Context) (BrowserState, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.index < 0 {
		return session.state(), errors.New("there is no document to reload")
	}
	result, err := session.exchange(ctx, session.history[session.index].URI)
	if err != nil {
		return session.state(), err
	}
	session.history[session.index] = result
	return session.state(), nil
}

func (session *BrowserSession) Snapshot() BrowserState {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.state()
}

func (session *BrowserSession) move(ctx context.Context, offset int) (BrowserState, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	target := session.index + offset
	if target < 0 || target >= len(session.history) {
		return session.state(), errors.New("no document in that history direction")
	}
	result, err := session.exchange(ctx, session.history[target].URI)
	if err != nil {
		return session.state(), err
	}
	session.history[target] = result
	session.index = target
	return session.state(), nil
}

func (session *BrowserSession) exchange(ctx context.Context, uri string) (NavigationResult, error) {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "mailweb" || parsed.Host == "" {
		return NavigationResult{}, errors.New("address must be an absolute mailweb:// URI")
	}
	request, err := NewRequest(parsed.String())
	if err != nil {
		return NavigationResult{}, err
	}
	sent := time.Now().UTC()
	response, err := session.transport.Exchange(ctx, request)
	received := time.Now().UTC()
	if err != nil {
		return NavigationResult{}, err
	}
	if err := ValidateResponse(request, response); err != nil {
		return NavigationResult{}, err
	}
	return NavigationResult{
		URI: parsed.String(), Request: request, Response: response,
		Transport: session.transportName, RequestSentAt: sent,
		ResponseReceivedAt: received, RoundTripMS: received.Sub(sent).Milliseconds(),
		ClientMailbox: session.clientMailbox,
	}, nil
}

func (session *BrowserSession) state() BrowserState {
	state := BrowserState{
		CanGoBack:         session.index > 0,
		CanGoForward:      session.index >= 0 && session.index < len(session.history)-1,
		SelectedTransport: session.transportName,
		ClientMailbox:     session.clientMailbox,
	}
	if session.index >= 0 {
		current := session.history[session.index]
		state.Current = &current
	}
	return state
}

func prettyJSON(value any) string {
	encoded, _ := json.MarshalIndent(value, "", "  ")
	return string(encoded)
}
