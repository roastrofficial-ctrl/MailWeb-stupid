package main

import (
	"bytes"
	"context"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeTransport struct {
	mu        sync.Mutex
	requests  []MailWebRequest
	documents map[string][]Node
}

func (transport *fakeTransport) Exchange(_ context.Context, request MailWebRequest) (MailWebResponse, error) {
	transport.mu.Lock()
	transport.requests = append(transport.requests, request)
	transport.mu.Unlock()
	path := strings.TrimPrefix(request.URI, "mailweb://demo.local")
	if transport.documents != nil {
		return MailWebResponse{MailWeb: request.MailWeb, RequestID: request.ID, Status: 200, Document: &MailWebDocument{Title: path, Body: transport.documents[path]}}, nil
	}
	if path == "/" {
		return MailWebResponse{MailWeb: request.MailWeb, RequestID: request.ID, Status: 200, Document: &MailWebDocument{
			Title: "Home", Body: []Node{{Type: "heading", Level: 1, Text: "Hello"}, {Type: "link", Label: "About", Href: "/about"}},
		}}, nil
	}
	return MailWebResponse{MailWeb: request.MailWeb, RequestID: request.ID, Status: 200, Document: &MailWebDocument{
		Title: "About", Body: []Node{{Type: "heading", Level: 1, Text: "About MailWeb"}},
	}}, nil
}

func TestBrowseNavigatesBetweenMailWebDocuments(t *testing.T) {
	transport := &fakeTransport{}
	var output bytes.Buffer
	if err := Browse(context.Background(), transport, "mailweb://demo.local/", strings.NewReader("1\n"), &output); err != nil {
		t.Fatal(err)
	}
	if len(transport.requests) != 2 || transport.requests[1].URI != "mailweb://demo.local/about" {
		t.Fatalf("unexpected requests: %#v", transport.requests)
	}
	if !strings.Contains(output.String(), "Hello") || !strings.Contains(output.String(), "About MailWeb") {
		t.Fatalf("documents were not rendered: %s", output.String())
	}
}

func TestValidateResponseRejectsUnknownNodesAndBadCorrelation(t *testing.T) {
	request, _ := NewRequest("mailweb://demo.local/")
	response := MailWebResponse{MailWeb: "0.1", RequestID: "wrong", Status: 200, Document: &MailWebDocument{}}
	if ValidateResponse(request, response) == nil {
		t.Fatal("bad correlation was accepted")
	}
	response.RequestID = request.ID
	response.Document.Body = []Node{{Type: "html", Text: "<script>"}}
	if ValidateResponse(request, response) == nil {
		t.Fatal("unknown node was accepted")
	}
}

func TestParseMailWebResponseRequiresMediaTypeAndDecodesJSON(t *testing.T) {
	raw := buildMailMessage("browse@demo.local", "postbox@client.local", "MailWeb response", []byte(`{"mailweb":"0.1","request_id":"01J00000000000000000000000","status":200,"document":{"title":"Hello","body":[]}}`))
	response, err := parseMailWebResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if response.RequestID != "01J00000000000000000000000" || response.Document == nil || response.Document.Title != "Hello" {
		t.Fatalf("unexpected response: %#v", response)
	}

	invalid := strings.Replace(string(raw), "application/mailweb+json", "text/plain", 1)
	if _, err := parseMailWebResponse([]byte(invalid)); err == nil {
		t.Fatal("non-MailWeb media type was accepted")
	}
}

func TestBrowserSessionMaintainsBackForwardAndReloadHistory(t *testing.T) {
	transport := &fakeTransport{}
	session := NewBrowserSession(transport, "test")
	session.DisablePrefetch()
	state, err := session.Navigate(context.Background(), "mailweb://demo.local/")
	if err != nil || state.CanGoBack || state.CanGoForward {
		t.Fatalf("unexpected initial state: %#v, %v", state, err)
	}
	state, err = session.NavigateReference(context.Background(), "/about")
	if err != nil || !state.CanGoBack || state.CanGoForward {
		t.Fatalf("unexpected second state: %#v, %v", state, err)
	}
	state, err = session.Back(context.Background())
	if err != nil || state.Current.URI != "mailweb://demo.local/" || !state.CanGoForward {
		t.Fatalf("back failed: %#v, %v", state, err)
	}
	state, err = session.Forward(context.Background())
	if err != nil || state.Current.URI != "mailweb://demo.local/about" || !state.CanGoBack {
		t.Fatalf("forward failed: %#v, %v", state, err)
	}
	before := state.Current.Request.ID
	state, err = session.Reload(context.Background())
	if err != nil || state.Current.Request.ID == before {
		t.Fatalf("reload did not make a fresh request: %#v, %v", state, err)
	}
}

func TestGraphicalRendererNeverUsesInnerHTML(t *testing.T) {
	assets, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		t.Fatal(err)
	}
	script, err := fs.ReadFile(assets, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(script)), "innerhtml") {
		t.Fatal("graphical renderer must not use innerHTML")
	}
	for _, required := range []string{"createElement", "textContent", "replaceChildren"} {
		if !strings.Contains(string(script), required) {
			t.Fatalf("graphical renderer does not use safe DOM operation %s", required)
		}
	}
}

func TestProtocol03GETQueryPOSTBodyAndForm(t *testing.T) {
	get, err := NewRequest("mailweb://demo.local/search?q=internet")
	if err != nil || get.URI != "mailweb://demo.local/search?q=internet" || get.Body != nil {
		t.Fatalf("unexpected GET request: %#v, %v", get, err)
	}
	post, err := NewRequestWithBody("POST", "mailweb://demo.local/hello", map[string]any{"name": "Levi"})
	if err != nil || post.Headers["content-type"] != "application/json" || post.Body["name"] != "Levi" {
		t.Fatalf("unexpected POST request: %#v, %v", post, err)
	}
	encoded := prettyJSON(post)
	if !strings.Contains(encoded, `"body": {`) || !strings.Contains(encoded, `"name": "Levi"`) {
		t.Fatalf("POST body was not serialized: %s", encoded)
	}
	form := Node{Type: "form", Method: "POST", Action: "/hello", Submit: "Send by post", Fields: []FormField{{Name: "name", Type: "text", Label: "Name"}}}
	if err := validateNode(form); err != nil {
		t.Fatalf("valid form rejected: %v", err)
	}
	form.Fields[0].Type = "password"
	if err := validateNode(form); err == nil {
		t.Fatal("unsupported form field type accepted")
	}
}

func TestPresentationIntentValidation(t *testing.T) {
	request, _ := NewRequest("mailweb://demo.local/")
	response := MailWebResponse{MailWeb: request.MailWeb, RequestID: request.ID, Status: 200, Document: &MailWebDocument{
		Title: "Themed", Presentation: &Presentation{Accent: "#315C45", Background: "#FFFDF8", Foreground: "#17231C", Surface: "#F3EFE5", Typeface: "editorial", Density: "spacious", Corners: "soft"},
		Body: []Node{{Type: "heading", Level: 1, Text: "Hello", Variant: "display"}, {Type: "button", Label: "Open", Href: "/", Variant: "prominent"}},
	}}
	if err := ValidateResponse(request, response); err != nil {
		t.Fatalf("valid presentation rejected: %v", err)
	}
	response.Document.Presentation.Accent = "url(javascript:evil)"
	if err := ValidateResponse(request, response); err == nil {
		t.Fatal("unsafe presentation accepted")
	}
}

func TestFormSubmissionBuildsGETQueryAndPOSTBody(t *testing.T) {
	transport := &fakeTransport{}
	session := NewBrowserSession(transport, "test")
	session.DisablePrefetch()
	if _, err := session.Navigate(context.Background(), "mailweb://demo.local/"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.SubmitForm(context.Background(), "GET", "/search?existing=yes", map[string]string{"q": "private mail"}); err != nil {
		t.Fatal(err)
	}
	if _, err := session.SubmitForm(context.Background(), "POST", "/hello", map[string]string{"name": "Levi"}); err != nil {
		t.Fatal(err)
	}
	transport.mu.Lock()
	requests := append([]MailWebRequest(nil), transport.requests...)
	transport.mu.Unlock()
	if requests[1].URI != "mailweb://demo.local/search?existing=yes&q=private+mail" || requests[1].Method != "GET" {
		t.Fatalf("bad GET form request: %#v", requests[1])
	}
	if requests[2].Method != "POST" || requests[2].Body["name"] != "Levi" {
		t.Fatalf("bad POST form request: %#v", requests[2])
	}
}

func TestPrEmailPrefetchRulesAndNoRecursion(t *testing.T) {
	transport := &fakeTransport{documents: map[string][]Node{
		"/": {
			{Type: "link", Label: "A", Href: "/a"}, {Type: "link", Label: "A duplicate", Href: "/a"},
			{Type: "link", Label: "B", Href: "/b"}, {Type: "link", Label: "C", Href: "/c"},
			{Type: "link", Label: "Over limit", Href: "/d"}, {Type: "link", Label: "External", Href: "mailweb://other.local/x"},
			{Type: "button", Label: "Mutation-like action", Href: "/button"},
		},
		"/a": {{Type: "link", Label: "Must not recurse", Href: "/nested"}}, "/b": {}, "/c": {},
	}}
	session := NewBrowserSession(transport, "test")
	if _, err := session.Navigate(context.Background(), "mailweb://demo.local/"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, time.Second, func() bool { return requestCount(transport) == 4 })
	time.Sleep(20 * time.Millisecond)
	transport.mu.Lock()
	requests := append([]MailWebRequest(nil), transport.requests...)
	transport.mu.Unlock()
	counts := map[string]int{}
	for _, request := range requests {
		counts[request.URI]++
	}
	if counts["mailweb://demo.local/a"] != 1 || counts["mailweb://demo.local/b"] != 1 || counts["mailweb://demo.local/c"] != 1 {
		t.Fatalf("eligible links not fetched once: %#v", counts)
	}
	for _, forbidden := range []string{"mailweb://demo.local/d", "mailweb://other.local/x", "mailweb://demo.local/button", "mailweb://demo.local/nested"} {
		if counts[forbidden] != 0 {
			t.Fatalf("forbidden URI was prefetched: %s", forbidden)
		}
	}
}

func TestPrEmailCacheHitExpiryAndReloadBypass(t *testing.T) {
	transport := &fakeTransport{documents: map[string][]Node{"/": {{Type: "link", Label: "A", Href: "/a"}}, "/a": {}}}
	session := NewBrowserSession(transport, "test")
	session.cacheTTL = 40 * time.Millisecond
	if _, err := session.Navigate(context.Background(), "mailweb://demo.local/"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, time.Second, func() bool { return requestCount(transport) == 2 })
	state, err := session.NavigateReference(context.Background(), "/a")
	if err != nil || state.Current.Delivery != "prEmail cache" || requestCount(transport) != 2 {
		t.Fatalf("cache hit failed: %#v, %v", state.Current, err)
	}
	if len(state.Archive) == 0 || state.Archive[0].URI != "mailweb://demo.local/a" {
		t.Fatalf("cached correspondence missing from archive: %#v", state.Archive)
	}
	if _, err := session.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if requestCount(transport) != 3 {
		t.Fatal("reload did not bypass cache")
	}
	if _, err := session.Navigate(context.Background(), "mailweb://demo.local/"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)
	before := requestCount(transport)
	if _, err := session.Navigate(context.Background(), "mailweb://demo.local/a"); err != nil {
		t.Fatal(err)
	}
	if requestCount(transport) != before+1 {
		t.Fatal("expired cache did not cause live request")
	}
}

func TestPrEmailNeverRunsForPOSTNavigation(t *testing.T) {
	transport := &fakeTransport{documents: map[string][]Node{"/": {{Type: "link", Label: "A", Href: "/a"}}}}
	session := NewBrowserSession(transport, "test")
	session.mu.Lock()
	session.history = []NavigationResult{{URI: "mailweb://demo.local/", Request: MailWebRequest{Method: "GET"}}}
	session.index = 0
	session.mu.Unlock()
	if _, err := session.SubmitForm(context.Background(), "POST", "/", map[string]string{"name": "Levi"}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	if requestCount(transport) != 1 {
		t.Fatalf("POST caused speculative requests: %d", requestCount(transport))
	}
}

func TestExhibitUIKeepsPresentationAndCorrespondenceLocal(t *testing.T) {
	assets, _ := fs.Sub(uiFiles, "ui")
	script, _ := fs.ReadFile(assets, "app.js")
	markup, _ := fs.ReadFile(assets, "index.html")
	styles, _ := fs.ReadFile(assets, "styles.css")
	combined := string(script) + string(markup) + string(styles)
	for _, required := range []string{"CorrespondenceAnimation", "PresentationResolver", "PostboxDrawer", "CorrespondenceView", "JourneyInspector", "postalState", "requestEmail", "responseEmail", "prefers-reduced-motion"} {
		if !strings.Contains(combined, required) {
			t.Fatalf("exhibit UI missing %s", required)
		}
	}
	if !strings.Contains(string(script), `presentation.apply(currentState.current.response.document || {}, appearanceMode)`) {
		t.Fatal("appearance changes do not locally re-resolve presentation")
	}
}

func TestJourneyRecordsTruthfulOrderedNavigationEvents(t *testing.T) {
	session := NewBrowserSession(&fakeTransport{}, "test")
	session.DisablePrefetch()
	state, err := session.Navigate(context.Background(), "mailweb://demo.local/")
	if err != nil || state.LastJourney == nil {
		t.Fatalf("journey missing: %#v, %v", state.LastJourney, err)
	}
	want := []string{"navigation.started", "cache.checked", "cache.miss", "request.created", "response.received", "response.correlated", "response.validated", "document.ready"}
	if len(state.LastJourney.Events) != len(want) {
		t.Fatalf("unexpected events: %#v", state.LastJourney.Events)
	}
	for index, event := range state.LastJourney.Events {
		if event.Type != want[index] { t.Fatalf("event %d: got %s, want %s", index, event.Type, want[index]) }
		if index > 0 && event.Timestamp.Before(state.LastJourney.Events[index-1].Timestamp) { t.Fatal("journey timestamps are not monotonic") }
	}
	if state.Current.JourneyID != state.LastJourney.ID || state.LastJourney.Request.ID != state.Current.Request.ID || state.LastJourney.Outcome != "delivered" {
		t.Fatalf("journey does not correlate with navigation: %#v", state.LastJourney)
	}
}

func TestJourneyCacheRetrievalLinksOriginalCorrespondence(t *testing.T) {
	session := NewBrowserSession(&fakeTransport{}, "test")
	session.DisablePrefetch()
	first, _ := session.Navigate(context.Background(), "mailweb://demo.local/")
	second, _ := session.Navigate(context.Background(), "mailweb://demo.local/")
	if second.LastJourney.OriginalJourney != first.Current.JourneyID || second.Current.Delivery != "correspondence archive" {
		t.Fatalf("archive journey lost provenance: %#v", second.LastJourney)
	}
	foundHit := false
	for _, event := range second.LastJourney.Events { if event.Type == "cache.hit" { foundHit = true } }
	if !foundHit { t.Fatal("cache retrieval was not recorded") }
}

func TestPOSTJourneyRedactsSubmittedValues(t *testing.T) {
	session := NewBrowserSession(&fakeTransport{}, "test")
	session.DisablePrefetch()
	_, _ = session.Navigate(context.Background(), "mailweb://demo.local/")
	state, err := session.SubmitForm(context.Background(), "POST", "/", map[string]string{"secret": "never-log-me"})
	if err != nil { t.Fatal(err) }
	encoded := prettyJSON(state.LastJourney.Events)
	if strings.Contains(encoded, "never-log-me") || !strings.Contains(encoded, `"values": "redacted"`) { t.Fatalf("unsafe journey metadata: %s", encoded) }
}

func requestCount(transport *fakeTransport) int {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	return len(transport.requests)
}
func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}
