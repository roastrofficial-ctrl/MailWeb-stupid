package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const defaultCacheTTL = 60 * time.Second

type NavigationResult struct {
	URI                string          `json:"uri"`
	Request            MailWebRequest  `json:"request"`
	Response           MailWebResponse `json:"response"`
	Transport          string          `json:"transport"`
	Delivery           string          `json:"delivery"`
	RequestSentAt      time.Time       `json:"request_sent_at"`
	ResponseReceivedAt time.Time       `json:"response_received_at"`
	RoundTripMS        int64           `json:"round_trip_ms"`
	PrefetchedAt       *time.Time      `json:"prefetched_at,omitempty"`
	OpenedAt           time.Time       `json:"opened_at"`
	NavigationMS       int64           `json:"navigation_ms"`
	ClientMailbox      string          `json:"client_mailbox,omitempty"`
}

type PrefetchState struct {
	Phase     string `json:"phase"`
	Active    int    `json:"active"`
	Completed int    `json:"completed"`
	Cached    int    `json:"cached"`
	Message   string `json:"message"`
}

type BrowserState struct {
	Current           *NavigationResult `json:"current"`
	CanGoBack         bool              `json:"can_go_back"`
	CanGoForward      bool              `json:"can_go_forward"`
	SelectedTransport string            `json:"selected_transport"`
	ClientMailbox     string            `json:"client_mailbox,omitempty"`
	Prefetch          PrefetchState     `json:"premail"`
	Notice            string            `json:"notice,omitempty"`
}

type cacheEntry struct {
	result    NavigationResult
	fetchedAt time.Time
	expiresAt time.Time
}

// BrowserSession owns navigation, history and experimental prEmail state.
// Transport remains the only boundary that knows how messages are carried.
type BrowserSession struct {
	mu              sync.Mutex
	transport       Transport
	transportName   string
	clientMailbox   string
	history         []NavigationResult
	index           int
	cache           map[string]cacheEntry
	inflight        map[string]chan struct{}
	cacheTTL        time.Duration
	prefetch        PrefetchState
	lastNotice      string
	prefetchEnabled bool
}

func NewBrowserSession(transport Transport, transportName string) *BrowserSession {
	mailbox := ""
	if provider, ok := transport.(interface{ Mailbox() string }); ok {
		mailbox = provider.Mailbox()
	}
	return &BrowserSession{
		transport: transport, transportName: transportName, clientMailbox: mailbox, index: -1,
		cache: map[string]cacheEntry{}, inflight: map[string]chan struct{}{}, cacheTTL: defaultCacheTTL,
		prefetchEnabled: true,
		prefetch:        PrefetchState{Phase: "idle", Message: "prEmail: waiting for addresses."},
	}
}

func (session *BrowserSession) DisablePrefetch() {
	session.mu.Lock()
	session.prefetchEnabled = false
	session.mu.Unlock()
}

func (session *BrowserSession) Navigate(ctx context.Context, uri string) (BrowserState, error) {
	return session.navigate(ctx, "GET", uri, nil, false, historyPush, true)
}

func (session *BrowserSession) NavigateReference(ctx context.Context, reference string) (BrowserState, error) {
	session.mu.Lock()
	if session.index < 0 {
		state := session.stateLocked()
		session.mu.Unlock()
		return state, errors.New("cannot resolve a relative link without a current document")
	}
	base := session.history[session.index].URI
	session.mu.Unlock()
	uri, err := resolveMailWebReference(base, reference)
	if err != nil {
		return session.Snapshot(), err
	}
	return session.navigate(ctx, "GET", uri, nil, false, historyPush, true)
}

func (session *BrowserSession) SubmitForm(ctx context.Context, method, action string, values map[string]string) (BrowserState, error) {
	session.mu.Lock()
	if session.index < 0 {
		state := session.stateLocked()
		session.mu.Unlock()
		return state, errors.New("cannot submit a form without a current document")
	}
	base := session.history[session.index].URI
	session.mu.Unlock()
	uri, err := resolveMailWebReference(base, action)
	if err != nil {
		return session.Snapshot(), err
	}
	method = strings.ToUpper(method)
	if method == "GET" {
		parsed, _ := url.Parse(uri)
		query := parsed.Query()
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			query.Set(key, values[key])
		}
		parsed.RawQuery = query.Encode()
		return session.navigate(ctx, "GET", parsed.String(), nil, false, historyPush, true)
	}
	if method != "POST" {
		return session.Snapshot(), errors.New("form method must be GET or POST")
	}
	body := make(map[string]any, len(values))
	for key, value := range values {
		body[key] = value
	}
	return session.navigate(ctx, "POST", uri, body, true, historyPush, true)
}

func (session *BrowserSession) Back(ctx context.Context) (BrowserState, error) {
	return session.move(ctx, -1)
}
func (session *BrowserSession) Forward(ctx context.Context) (BrowserState, error) {
	return session.move(ctx, 1)
}

func (session *BrowserSession) Reload(ctx context.Context) (BrowserState, error) {
	session.mu.Lock()
	if session.index < 0 {
		state := session.stateLocked()
		session.mu.Unlock()
		return state, errors.New("there is no document to reload")
	}
	current := session.history[session.index]
	session.mu.Unlock()
	body := current.Request.Body
	return session.navigate(ctx, current.Request.Method, current.URI, body, true, historyReplace, true)
}

func (session *BrowserSession) Snapshot() BrowserState {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.stateLocked()
}

type historyMode int

const (
	historyPush historyMode = iota
	historyReplace
)

func (session *BrowserSession) move(ctx context.Context, offset int) (BrowserState, error) {
	session.mu.Lock()
	target := session.index + offset
	if target < 0 || target >= len(session.history) {
		state := session.stateLocked()
		session.mu.Unlock()
		return state, errors.New("no document in that history direction")
	}
	previousIndex := session.index
	stored := session.history[target]
	session.index = target
	session.mu.Unlock()
	state, err := session.navigate(ctx, stored.Request.Method, stored.URI, stored.Request.Body, false, historyReplace, true)
	if err != nil {
		session.mu.Lock()
		session.index = previousIndex
		state = session.stateLocked()
		session.mu.Unlock()
	}
	return state, err
}

func (session *BrowserSession) navigate(ctx context.Context, method, uri string, body map[string]any, bypass bool, mode historyMode, discover bool) (BrowserState, error) {
	canonical, err := canonicalMailWebURI(uri)
	if err != nil {
		return session.Snapshot(), err
	}
	started := time.Now()
	var result NavigationResult
	if method == "GET" && !bypass {
		result, _ = session.awaitCached(ctx, canonical)
	}
	if result.Request.ID == "" {
		result, err = session.exchange(ctx, method, canonical, body)
		if err != nil {
			return session.Snapshot(), err
		}
		result.NavigationMS = time.Since(started).Milliseconds()
	} else {
		result.Delivery = "prEmail cache"
		result.OpenedAt = time.Now().UTC()
		result.NavigationMS = time.Since(started).Milliseconds()
	}
	session.mu.Lock()
	if result.Delivery == "prEmail cache" {
		session.lastNotice = "Already in your postbox — opened instantly."
	} else {
		session.lastNotice = ""
	}
	if mode == historyPush {
		session.history = append(session.history[:session.index+1], result)
		session.index = len(session.history) - 1
	} else {
		session.history[session.index] = result
	}
	state := session.stateLocked()
	session.mu.Unlock()
	if discover {
		session.discover(result)
	}
	return state, nil
}

func (session *BrowserSession) exchange(ctx context.Context, method, uri string, body map[string]any) (NavigationResult, error) {
	request, err := NewRequestWithBody(method, uri, body)
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
		URI: uri, Request: request, Response: response, Transport: session.transportName, Delivery: "live",
		RequestSentAt: sent, ResponseReceivedAt: received, RoundTripMS: received.Sub(sent).Milliseconds(),
		OpenedAt: received, NavigationMS: received.Sub(sent).Milliseconds(), ClientMailbox: session.clientMailbox,
	}, nil
}

func (session *BrowserSession) awaitCached(ctx context.Context, uri string) (NavigationResult, bool) {
	for {
		session.mu.Lock()
		if entry, ok := session.cache[uri]; ok {
			if time.Now().Before(entry.expiresAt) {
				result := entry.result
				session.mu.Unlock()
				return result, true
			}
			delete(session.cache, uri)
		}
		wait, active := session.inflight[uri]
		session.mu.Unlock()
		if !active {
			return NavigationResult{}, false
		}
		select {
		case <-wait:
			continue
		case <-ctx.Done():
			return NavigationResult{}, false
		}
	}
}

func (session *BrowserSession) discover(current NavigationResult) {
	session.mu.Lock()
	enabled := session.prefetchEnabled
	session.mu.Unlock()
	if !enabled {
		return
	}
	if current.Request.Method != "GET" || current.Response.Document == nil {
		return
	}
	targets := make([]string, 0, 3)
	seen := map[string]bool{}
	base, _ := url.Parse(current.URI)
	for _, node := range current.Response.Document.Body {
		if node.Type != "link" {
			continue
		}
		resolved, err := resolveMailWebReference(current.URI, node.Href)
		if err != nil {
			continue
		}
		parsed, _ := url.Parse(resolved)
		if !strings.EqualFold(parsed.Host, base.Host) || resolved == current.URI || seen[resolved] {
			continue
		}
		seen[resolved] = true
		session.mu.Lock()
		entry, cached := session.cache[resolved]
		_, active := session.inflight[resolved]
		if cached && time.Now().After(entry.expiresAt) {
			delete(session.cache, resolved)
			cached = false
		}
		if !cached && !active {
			session.inflight[resolved] = make(chan struct{})
			targets = append(targets, resolved)
		}
		session.mu.Unlock()
		if len(targets) == 3 {
			break
		}
	}
	if len(targets) == 0 {
		session.refreshPrefetchStatus()
		return
	}
	session.mu.Lock()
	session.prefetch.Phase = "fetching"
	session.prefetch.Active += len(targets)
	session.prefetch.Completed = 0
	session.prefetch.Message = prefetchAwaitingMessage(session.prefetch.Active)
	session.mu.Unlock()
	for _, target := range targets {
		go session.prefetchURI(target)
	}
}

func (session *BrowserSession) prefetchURI(uri string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result, err := session.exchange(ctx, "GET", uri, nil)
	now := time.Now().UTC()
	session.mu.Lock()
	if err == nil && result.Response.Status >= 200 && result.Response.Status < 300 {
		result.PrefetchedAt = &now
		session.cache[uri] = cacheEntry{result: result, fetchedAt: now, expiresAt: now.Add(session.cacheTTL)}
		session.prefetch.Completed++
	}
	if wait, ok := session.inflight[uri]; ok {
		close(wait)
		delete(session.inflight, uri)
	}
	session.prefetch.Active--
	session.updatePrefetchStatusLocked()
	session.mu.Unlock()
}

func (session *BrowserSession) refreshPrefetchStatus() {
	session.mu.Lock()
	session.updatePrefetchStatusLocked()
	session.mu.Unlock()
}
func (session *BrowserSession) updatePrefetchStatusLocked() {
	now := time.Now()
	cached := 0
	for key, entry := range session.cache {
		if now.Before(entry.expiresAt) {
			cached++
		} else {
			delete(session.cache, key)
		}
	}
	session.prefetch.Cached = cached
	if session.prefetch.Active > 0 {
		session.prefetch.Phase = "fetching"
		session.prefetch.Message = prefetchAwaitingMessage(session.prefetch.Active)
		return
	}
	if cached > 0 {
		session.prefetch.Phase = "ready"
		session.prefetch.Message = "prEmail: " + plural(cached, "likely destination", "likely destinations") + " ready in your postbox."
		return
	}
	session.prefetch.Phase = "idle"
	session.prefetch.Message = "prEmail: no correspondence waiting ahead."
}

func prefetchAwaitingMessage(count int) string {
	return "prEmail: awaiting " + plural(count, "reply", "replies") + "…"
}
func plural(count int, one, many string) string {
	word := many
	if count == 1 {
		word = one
	}
	return jsonNumber(count) + " " + word
}
func jsonNumber(value int) string { encoded, _ := json.Marshal(value); return string(encoded) }

func canonicalMailWebURI(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(parsed.Scheme, "mailweb") || parsed.Host == "" || parsed.User != nil {
		return "", errors.New("address must be an absolute mailweb:// URI")
	}
	parsed.Scheme = "mailweb"
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	parsed.RawQuery = parsed.Query().Encode()
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), nil
}

func (session *BrowserSession) stateLocked() BrowserState {
	session.updatePrefetchStatusLocked()
	state := BrowserState{CanGoBack: session.index > 0, CanGoForward: session.index >= 0 && session.index < len(session.history)-1,
		SelectedTransport: session.transportName, ClientMailbox: session.clientMailbox, Prefetch: session.prefetch, Notice: session.lastNotice}
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
