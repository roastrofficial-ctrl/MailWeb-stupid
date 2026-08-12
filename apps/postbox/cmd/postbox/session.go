package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

const defaultCacheTTL = 60 * time.Second

type NavigationResult struct {
	JourneyID          string           `json:"journey_id"`
	URI                string           `json:"uri"`
	Request            MailWebRequest   `json:"request"`
	Response           MailWebResponse  `json:"response"`
	RawResponse        *MailWebResponse `json:"raw_response,omitempty"`
	StationeryStatus   string           `json:"stationery_status,omitempty"`
	EnclosuresReceived int              `json:"enclosures_received"`
	EnclosuresReused   int              `json:"enclosures_reused"`
	EnclosureBytes     int64            `json:"enclosure_bytes"`
	Transport          string           `json:"transport"`
	Delivery           string           `json:"delivery"`
	RequestSentAt      time.Time        `json:"request_sent_at"`
	ResponseReceivedAt time.Time        `json:"response_received_at"`
	RoundTripMS        int64            `json:"round_trip_ms"`
	PrefetchedAt       *time.Time       `json:"prefetched_at,omitempty"`
	OpenedAt           time.Time        `json:"opened_at"`
	NavigationMS       int64            `json:"navigation_ms"`
	ClientMailbox      string           `json:"client_mailbox,omitempty"`
	PublisherMailbox   string           `json:"publisher_mailbox,omitempty"`
}

type PrefetchState struct {
	Phase     string   `json:"phase"`
	Active    int      `json:"active"`
	Completed int      `json:"completed"`
	Cached    int      `json:"cached"`
	Message   string   `json:"message"`
	Targets   []string `json:"targets,omitempty"`
}

type BrowserState struct {
	Current                *NavigationResult `json:"current"`
	CanGoBack              bool              `json:"can_go_back"`
	CanGoForward           bool              `json:"can_go_forward"`
	SelectedTransport      string            `json:"selected_transport"`
	ClientMailbox          string            `json:"client_mailbox,omitempty"`
	Prefetch               PrefetchState     `json:"premail"`
	Notice                 string            `json:"notice,omitempty"`
	Archive                []ArchiveItem     `json:"archive"`
	Journeys               []Journey         `json:"journeys"`
	LastJourney            *Journey          `json:"last_journey,omitempty"`
	Stationery             []StationeryFile  `json:"stationery"`
	Enclosures             []EnclosureFile   `json:"enclosures"`
	RetainedCorrespondence *MailWebResponse  `json:"retained_correspondence,omitempty"`
}

type ArchiveItem struct {
	URI         string    `json:"uri"`
	Title       string    `json:"title"`
	ReceivedAt  time.Time `json:"received_at"`
	Delivery    string    `json:"delivery"`
	Current     bool      `json:"current"`
	RoundTripMS int64     `json:"round_trip_ms"`
}

type cacheEntry struct {
	result    NavigationResult
	fetchedAt time.Time
	expiresAt time.Time
}

// BrowserSession owns navigation, history and experimental prEmail state.
// Transport remains the only boundary that knows how messages are carried.
type BrowserSession struct {
	mu                     sync.Mutex
	transport              Transport
	transportName          string
	clientMailbox          string
	publisherMailbox       string
	history                []NavigationResult
	index                  int
	cache                  map[string]cacheEntry
	inflight               map[string]chan struct{}
	cacheTTL               time.Duration
	prefetch               PrefetchState
	lastNotice             string
	prefetchEnabled        bool
	journeys               []Journey
	stationery             map[string]StationeryFile
	enclosures             map[string]EnclosureFile
	retainedCorrespondence *MailWebResponse
}

func NewBrowserSession(transport Transport, transportName string) *BrowserSession {
	mailbox := ""
	if provider, ok := transport.(interface{ Mailbox() string }); ok {
		mailbox = provider.Mailbox()
	}
	publisher := ""
	if correspondent, ok := transport.(interface{ Correspondent() string }); ok {
		publisher = correspondent.Correspondent()
	}
	return &BrowserSession{
		transport: transport, transportName: transportName, clientMailbox: mailbox, publisherMailbox: publisher, index: -1,
		cache: map[string]cacheEntry{}, inflight: map[string]chan struct{}{}, cacheTTL: defaultCacheTTL,
		prefetchEnabled: true,
		stationery:      map[string]StationeryFile{},
		enclosures:      map[string]EnclosureFile{},
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

func (session *BrowserSession) SubmitClientAction(ctx context.Context, action string, body map[string]any) (BrowserState, error) {
	session.mu.Lock()
	if session.index < 0 { state := session.stateLocked(); session.mu.Unlock(); return state, errors.New("cannot perform a client action without a current document") }
	base := session.history[session.index].URI
	session.mu.Unlock()
	uri, err := resolveMailWebReference(base, action); if err != nil { return session.Snapshot(), err }
	return session.navigate(ctx, "POST", uri, body, true, historyPush, true)
}

func (session *BrowserSession) AuthorizeClientAction(capability, action string, parameters map[string]string) error {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.index < 0 { return errors.New("no current document") }
	for _, node := range session.history[session.index].Response.Document.Body {
		if node.Type == "client_action" && node.Capability == capability && node.Action == action && reflect.DeepEqual(node.Parameters, parameters) { return nil }
	}
	return errors.New("client action was not requested by the current MailWeb document")
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
	journey := newJourney(canonical, method, session.transportName)
	journey.add("cache.checked", "Postbox checked for existing correspondence", map[string]string{"bypass": jsonBool(bypass)})
	if method == "POST" {
		journey.add("form.collected", "form enclosed for private delivery", map[string]string{"fields": jsonNumber(len(body)), "values": "redacted"})
		if _, ok := body["passport_proof"]; ok {
			journey.add("capability.delegated", "identity capability delegated to configured local provider", map[string]string{"provider":"Technical Passport Service", "transport":"HOST INTEGRATION"})
			journey.add("capability.result", "safe capability result received from local provider", map[string]string{"secret":"not received by Postbox"})
			journey.add("capability.returned", "portable identity proof enclosed through MailWeb", nil)
		}
	}
	var result NavigationResult
	if method == "GET" && !bypass {
		result, _ = session.awaitCached(ctx, canonical)
	}
	if result.Request.ID == "" {
		journey.add("cache.miss", "no current correspondence found", nil)
		result, err = session.exchange(ctx, method, canonical, body, journey)
		if err != nil {
			journey.add("navigation.error", "correspondence failed", map[string]string{"error": safeJourneyError(err)})
			completed := journey.finish(journeyOutcome(err, 0), "live correspondence", nil, nil, time.Since(started).Milliseconds(), time.Since(started).Milliseconds())
			session.storeJourney(completed)
			return session.Snapshot(), err
		}
		result.NavigationMS = time.Since(started).Milliseconds()
	} else {
		originalJourney := result.JourneyID
		journey.add("cache.hit", "correspondence found in Postbox", map[string]string{"original_journey_id": originalJourney})
		journey.journey.OriginalJourney = originalJourney
		if result.PrefetchedAt != nil {
			result.Delivery = "prEmail cache"
		} else {
			result.Delivery = "correspondence archive"
		}
		result.OpenedAt = time.Now().UTC()
		result.NavigationMS = time.Since(started).Milliseconds()
	}
	journey.add("document.ready", "semantic document released to renderer", nil)
	journeyResponse := &result.Response
	if result.RawResponse != nil {
		journeyResponse = result.RawResponse
	}
	completed := journey.finish(journeyOutcome(nil, result.Response.Status), result.Delivery, &result.Request, journeyResponse, result.RoundTripMS, result.NavigationMS)
	result.JourneyID = completed.ID
	session.storeJourney(completed)
	session.mu.Lock()
	if method == "GET" && result.Response.Status >= 200 && result.Response.Status < 300 && result.Delivery == "live" {
		now := time.Now().UTC()
		session.cache[canonical] = cacheEntry{result: result, fetchedAt: now, expiresAt: now.Add(session.cacheTTL)}
	}
	if result.Delivery != "live" {
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

func (session *BrowserSession) exchange(ctx context.Context, method, uri string, body map[string]any, journey *journeyRecorder) (NavigationResult, error) {
	request, err := NewRequestWithBody(method, uri, body)
	if err != nil {
		return NavigationResult{}, err
	}
	session.mu.Lock()
	known := make(map[string]string, len(session.stationery))
	for _, file := range session.stationery {
		known[file.ID] = file.Version
	}
	knownResources := make([]string, 0, len(session.enclosures))
	for digest := range session.enclosures {
		knownResources = append(knownResources, digest)
	}
	session.mu.Unlock()
	if len(known) > 0 {
		encoded, _ := json.Marshal(known)
		request.Headers["mailweb-stationery"] = string(encoded)
	}
	sort.Strings(knownResources)
	if len(knownResources) > 64 {
		knownResources = knownResources[len(knownResources)-64:]
	}
	if len(knownResources) > 0 {
		encoded, _ := json.Marshal(knownResources)
		request.Headers["mailweb-known-resources"] = string(encoded)
	}
	journey.add("request.created", "MailWebRequest created", map[string]string{"request_id": request.ID, "protocol": request.MailWeb})
	sent := time.Now().UTC()
	response, err := session.transport.Exchange(withJourneyObserver(ctx, journey), request)
	received := time.Now().UTC()
	if err != nil {
		return NavigationResult{}, err
	}
	journey.add("response.received", "correlated MailWebResponse received", map[string]string{"request_id": response.RequestID, "status": jsonNumber(response.Status)})
	journey.add("response.correlated", "response request_id matched outgoing correspondence", map[string]string{"request_id": response.RequestID})
	if err := ValidateResponse(request, response); err != nil {
		return NavigationResult{}, err
	}
	journey.add("response.validated", "protocol and correlation validated", nil)
	receivedEnclosures, reusedEnclosures := 0, 0
	var enclosureBytes int64
	for _, enclosure := range response.Enclosures {
		enclosureBytes += enclosure.Size
		if len(enclosure.Content) > 0 {
			receivedEnclosures++
		} else {
			reusedEnclosures++
		}
	}
	retainedResponse := response
	retainedResponse.Enclosures = append([]Enclosure(nil), response.Enclosures...)
	for index := range retainedResponse.Enclosures {
		retainedResponse.Enclosures[index].Content = nil
	}
	response, err = session.prepareEnclosures(response, uri, journey)
	if err != nil {
		if isMissingEnclosure(err) {
			session.mu.Lock()
			retained := retainedResponse
			session.retainedCorrespondence = &retained
			session.mu.Unlock()
		}
		return NavigationResult{}, err
	}
	rawResponse := response
	response, err = session.prepareResponse(response, uri, journey)
	if err != nil {
		if isMissingStationery(err) {
			session.mu.Lock()
			retained := rawResponse
			session.retainedCorrespondence = &retained
			session.mu.Unlock()
		}
		return NavigationResult{}, err
	}
	stationeryStatus := ""
	if rawResponse.Document.Template != "" {
		stationeryStatus = "Using stationery already in your Postbox."
		for _, template := range rawResponse.Templates {
			if template.ID == rawResponse.Document.Template && template.Version == rawResponse.Document.TemplateVersion {
				stationeryStatus = "New stationery received from " + mustMailWebHost(uri) + "."
			}
		}
	}
	publisherMailbox := session.publisherMailbox
	if correspondent, ok := session.transport.(interface{ CorrespondentFor(string) string }); ok {
		publisherMailbox = correspondent.CorrespondentFor(uri)
	}
	return NavigationResult{
		URI: uri, Request: request, Response: response, RawResponse: &rawResponse, StationeryStatus: stationeryStatus, EnclosuresReceived: receivedEnclosures, EnclosuresReused: reusedEnclosures, EnclosureBytes: enclosureBytes, Transport: session.transportName, Delivery: "live",
		RequestSentAt: sent, ResponseReceivedAt: received, RoundTripMS: received.Sub(sent).Milliseconds(),
		OpenedAt: received, NavigationMS: received.Sub(sent).Milliseconds(), ClientMailbox: session.clientMailbox,
		PublisherMailbox: publisherMailbox,
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
		references := []string{}
		if node.Type == "link" {
			references = append(references, node.Href)
		}
		if node.Type == "nav" {
			for _, item := range node.Items {
				references = append(references, item.Href)
			}
		}
		for _, reference := range references {
			resolved, err := resolveMailWebReference(current.URI, reference)
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
	session.prefetch.Targets = append(session.prefetch.Targets, targets...)
	session.prefetch.Message = prefetchAwaitingMessage(session.prefetch.Active)
	session.mu.Unlock()
	for _, target := range targets {
		go session.prefetchURI(target)
	}
}

func (session *BrowserSession) prefetchURI(uri string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	journey := newJourney(uri, "GET", session.transportName)
	journey.journey.Delivery = "prEmail"
	journey.add("premail.requested", "likely destination mailed in advance", nil)
	result, err := session.exchange(ctx, "GET", uri, nil, journey)
	now := time.Now().UTC()
	session.mu.Lock()
	if err == nil && result.Response.Status >= 200 && result.Response.Status < 300 {
		journey.add("premail.received", "reply filed unopened in Postbox", nil)
		completed := journey.finish("filed", "prEmail", &result.Request, &result.Response, result.RoundTripMS, result.RoundTripMS)
		result.JourneyID = completed.ID
		session.journeys = appendCappedJourney(session.journeys, completed)
		result.PrefetchedAt = &now
		session.cache[uri] = cacheEntry{result: result, fetchedAt: now, expiresAt: now.Add(session.cacheTTL)}
		session.prefetch.Completed++
	}
	if err != nil {
		journey.add("navigation.error", "prEmail correspondence failed", map[string]string{"error": safeJourneyError(err)})
		completed := journey.finish(journeyOutcome(err, 0), "prEmail", nil, nil, 0, 0)
		session.journeys = appendCappedJourney(session.journeys, completed)
	}
	if wait, ok := session.inflight[uri]; ok {
		close(wait)
		delete(session.inflight, uri)
	}
	session.prefetch.Active--
	for index, target := range session.prefetch.Targets {
		if target == uri {
			session.prefetch.Targets = append(session.prefetch.Targets[:index], session.prefetch.Targets[index+1:]...)
			break
		}
	}
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
			if entry.result.PrefetchedAt != nil {
				cached++
			}
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
func jsonBool(value bool) string  { encoded, _ := json.Marshal(value); return string(encoded) }

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
	state.Journeys = append([]Journey(nil), session.journeys...)
	if len(state.Journeys) > 0 {
		last := state.Journeys[len(state.Journeys)-1]
		state.LastJourney = &last
	}
	for _, file := range session.stationery {
		state.Stationery = append(state.Stationery, file)
	}
	for _, file := range session.enclosures {
		state.Enclosures = append(state.Enclosures, file)
	}
	state.RetainedCorrespondence = session.retainedCorrespondence
	sort.Slice(state.Stationery, func(i, j int) bool { return state.Stationery[i].ReceivedAt.After(state.Stationery[j].ReceivedAt) })
	sort.Slice(state.Enclosures, func(i, j int) bool { return state.Enclosures[i].ReceivedAt.After(state.Enclosures[j].ReceivedAt) })
	seen := map[string]bool{}
	if state.Current != nil {
		state.Archive = append(state.Archive, archiveItem(*state.Current, true))
		seen[state.Current.URI] = true
	}
	for uri, entry := range session.cache {
		if !seen[uri] && time.Now().Before(entry.expiresAt) {
			state.Archive = append(state.Archive, archiveItem(entry.result, false))
		}
	}
	sort.Slice(state.Archive, func(i, j int) bool { return state.Archive[i].ReceivedAt.After(state.Archive[j].ReceivedAt) })
	return state
}

func (session *BrowserSession) storeJourney(journey Journey) {
	session.mu.Lock()
	session.journeys = appendCappedJourney(session.journeys, journey)
	session.mu.Unlock()
}

func appendCappedJourney(journeys []Journey, journey Journey) []Journey {
	journeys = append(journeys, journey)
	if len(journeys) > 50 {
		journeys = journeys[len(journeys)-50:]
	}
	return journeys
}

func journeyOutcome(err error, status int) string {
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timed out") {
			return "timeout"
		}
		return "damaged"
	}
	if status >= 200 && status < 300 {
		return "delivered"
	}
	if status == 404 {
		return "returned"
	}
	if status >= 500 {
		return "publisher error"
	}
	return "response received"
}

func safeJourneyError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timed out") {
		return "timeout waiting for correlated response"
	}
	return err.Error()
}

func mustMailWebHost(uri string) string { parsed, _ := url.Parse(uri); return parsed.Host }

func archiveItem(result NavigationResult, current bool) ArchiveItem {
	title := result.URI
	if result.Response.Document != nil && result.Response.Document.Title != "" {
		title = result.Response.Document.Title
	}
	delivery := result.Delivery
	if delivery == "live" {
		delivery = "live correspondence"
	}
	if result.PrefetchedAt != nil {
		delivery = "prEmail"
	}
	return ArchiveItem{URI: result.URI, Title: title, ReceivedAt: result.ResponseReceivedAt, Delivery: delivery, Current: current, RoundTripMS: result.RoundTripMS}
}

func prettyJSON(value any) string {
	encoded, _ := json.MarshalIndent(value, "", "  ")
	return string(encoded)
}
