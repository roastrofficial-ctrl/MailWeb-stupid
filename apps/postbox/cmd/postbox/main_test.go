package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
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
		if event.Type != want[index] {
			t.Fatalf("event %d: got %s, want %s", index, event.Type, want[index])
		}
		if index > 0 && event.Timestamp.Before(state.LastJourney.Events[index-1].Timestamp) {
			t.Fatal("journey timestamps are not monotonic")
		}
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
	for _, event := range second.LastJourney.Events {
		if event.Type == "cache.hit" {
			foundHit = true
		}
	}
	if !foundHit {
		t.Fatal("cache retrieval was not recorded")
	}
}

func TestPOSTJourneyRedactsSubmittedValues(t *testing.T) {
	session := NewBrowserSession(&fakeTransport{}, "test")
	session.DisablePrefetch()
	_, _ = session.Navigate(context.Background(), "mailweb://demo.local/")
	state, err := session.SubmitForm(context.Background(), "POST", "/", map[string]string{"secret": "never-log-me"})
	if err != nil {
		t.Fatal(err)
	}
	encoded := prettyJSON(state.LastJourney.Events)
	if strings.Contains(encoded, "never-log-me") || !strings.Contains(encoded, `"values": "redacted"`) {
		t.Fatalf("unsafe journey metadata: %s", encoded)
	}
}

type stationeryTransport struct {
	template TemplateDefinition
	requests []MailWebRequest
}

func (transport *stationeryTransport) Exchange(_ context.Context, request MailWebRequest) (MailWebResponse, error) {
	transport.requests = append(transport.requests, request)
	document := &MailWebDocument{Title: "Page", Body: []Node{}, Template: transport.template.ID, TemplateVersion: transport.template.Version, Slots: map[string][]Node{"content": {{Type: "heading", Level: 2, Text: "Inserted letter"}}}}
	response := MailWebResponse{MailWeb: request.MailWeb, RequestID: request.ID, Status: 200, Document: document}
	if !strings.Contains(request.Headers["mailweb-stationery"], transport.template.Version) {
		response.Templates = []TemplateDefinition{transport.template}
	}
	return response, nil
}

func testStationery() TemplateDefinition {
	document := MailWebDocument{Title: "Stationery", Body: []Node{{Type: "heading", Level: 1, Text: "Dear Internet"}, {Type: "nav", Label: "Main", Items: []NavItem{{Label: "Home", Href: "/"}, {Label: "About", Href: "/about"}}}, {Type: "slot", Name: "content"}, {Type: "paragraph", Text: "Footer"}}, Presentation: &Presentation{Accent: "#315C45"}}
	return TemplateDefinition{ID: "dear-internet/site", Version: templateVersion(document), Document: document}
}

func TestStationeryFirstDeliveryCompositionAndReuse(t *testing.T) {
	transport := &stationeryTransport{template: testStationery()}
	session := NewBrowserSession(transport, "test")
	session.DisablePrefetch()
	first, err := session.Navigate(context.Background(), "mailweb://demo.local/")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Stationery) != 1 || len(first.Current.Response.Document.Body) != 4 || first.Current.Response.Document.Body[2].Text != "Inserted letter" || first.Current.StationeryStatus == "" {
		t.Fatalf("first composition failed: %#v", first)
	}
	second, err := session.Navigate(context.Background(), "mailweb://demo.local/about")
	if err != nil {
		t.Fatal(err)
	}
	if len(transport.requests) != 2 || !strings.Contains(transport.requests[1].Headers["mailweb-stationery"], transport.template.Version) || len(second.Current.RawResponse.Templates) != 0 {
		t.Fatalf("stationery was not reused: %#v", transport.requests)
	}
	if second.Current.Response.Document.Presentation.Accent != "#315C45" {
		t.Fatal("template presentation did not survive composition")
	}
}

func TestStationeryValidationAndMissingBehavior(t *testing.T) {
	template := testStationery()
	request, _ := NewRequest("mailweb://demo.local/")
	response := MailWebResponse{MailWeb: "0.5", RequestID: request.ID, Status: 200, Document: &MailWebDocument{Title: "Page", Body: []Node{}, Template: template.ID, TemplateVersion: template.Version, Slots: map[string][]Node{"unknown": {{Type: "paragraph", Text: "lost"}}}}, Templates: []TemplateDefinition{template}}
	if err := ValidateResponse(request, response); err != nil {
		t.Fatal(err)
	}
	session := NewBrowserSession(&stationeryTransport{template: template}, "test")
	journey := newJourney(request.URI, "GET", "test")
	if _, err := session.prepareResponse(response, request.URI, journey); err == nil || !strings.Contains(err.Error(), "unknown stationery slot") {
		t.Fatalf("unknown slot was accepted: %v", err)
	}
	missing := response
	missing.Templates = nil
	missing.Document.Slots = map[string][]Node{"content": {}}
	empty := NewBrowserSession(&fakeTransport{}, "test")
	if _, err := empty.prepareResponse(missing, request.URI, journey); !isMissingStationery(err) {
		t.Fatalf("missing stationery did not fail safely: %v", err)
	}
	bad := template
	bad.Version = "sha256:" + strings.Repeat("0", 64)
	response.Templates = []TemplateDefinition{bad}
	if ValidateResponse(request, response) == nil {
		t.Fatal("mismatched content identity was accepted")
	}
}

func TestTerminalRendersNavAsNavigation(t *testing.T) {
	var output bytes.Buffer
	response := MailWebResponse{Document: &MailWebDocument{Body: []Node{{Type: "nav", Label: "Main navigation", Items: []NavItem{{Label: "Home", Href: "/"}, {Label: "About", Href: "/about"}}}}}}
	links := RenderTerminal(&output, response)
	if len(links) != 2 || links[1].Href != "/about" || !strings.Contains(output.String(), "Main navigation") {
		t.Fatalf("terminal nav failed: %s %#v", output.String(), links)
	}
}

func TestNavAndSlotProtocolValidation(t *testing.T) {
	if err := validateNode(Node{Type: "nav", Label: "Main", Items: []NavItem{{Label: "Home", Href: "/"}}}); err != nil {
		t.Fatalf("valid nav rejected: %v", err)
	}
	if validateNode(Node{Type: "nav", Label: "Main", Items: []NavItem{{Label: "Bad", Href: "javascript:alert(1)"}}}) == nil {
		t.Fatal("unsafe nav item accepted")
	}
	if err := validateNode(Node{Type: "slot", Name: "content"}); err != nil {
		t.Fatalf("valid slot rejected: %v", err)
	}
	if validateNode(Node{Type: "slot", Name: "bad slot"}) == nil {
		t.Fatal("malformed slot accepted")
	}
}

func TestNavItemsParticipateInPrEmail(t *testing.T) {
	transport := &fakeTransport{documents: map[string][]Node{"/": {{Type: "nav", Label: "Main", Items: []NavItem{{Label: "About", Href: "/about"}}}}, "/about": {}}}
	session := NewBrowserSession(transport, "test")
	if _, err := session.Navigate(context.Background(), "mailweb://demo.local/"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, time.Second, func() bool { return requestCount(transport) == 2 })
	if transport.requests[1].URI != "mailweb://demo.local/about" {
		t.Fatalf("nav destination was not pre-mailed: %#v", transport.requests)
	}
}

func enclosureFixture() Enclosure {
	content := []byte("enclosed correspondence")
	sum := sha256.Sum256(content)
	return Enclosure{ID: "manifesto", Filename: "manifesto.txt", MediaType: "text/plain", Size: int64(len(content)), Digest: "sha256:" + hex.EncodeToString(sum[:]), Content: content}
}

func TestEnclosureValidationStorageAndReuse(t *testing.T) {
	request, _ := NewRequest("mailweb://demo.local/")
	enclosure := enclosureFixture()
	response := MailWebResponse{MailWeb: "0.5", RequestID: request.ID, Status: 200, Enclosures: []Enclosure{enclosure}, Document: &MailWebDocument{Title: "File", Body: []Node{{Type: "attachment", Enclosure: enclosure.ID, Digest: enclosure.Digest, Label: "Download"}}}}
	if err := ValidateResponse(request, response); err != nil {
		t.Fatal(err)
	}
	session := NewBrowserSession(&fakeTransport{}, "test")
	journey := newJourney(request.URI, "GET", "test")
	prepared, err := session.prepareEnclosures(response, request.URI, journey)
	if err != nil || len(prepared.Enclosures[0].Content) != 0 {
		t.Fatalf("enclosure not filed safely: %#v %v", prepared, err)
	}
	if stored, ok := session.Enclosure(enclosure.Digest); !ok || !bytes.Equal(stored.Content, enclosure.Content) {
		t.Fatal("binary bytes were not preserved")
	}
	reused := response
	reused.Enclosures[0].Content = nil
	if _, err := session.prepareEnclosures(reused, request.URI, journey); err != nil {
		t.Fatalf("known enclosure was not reused: %v", err)
	}
	damaged := response
	damaged.Enclosures[0].Content = []byte("tampered")
	if ValidateResponse(request, damaged) == nil {
		t.Fatal("digest mismatch accepted")
	}
	duplicate := response
	duplicate.Enclosures = append(duplicate.Enclosures, enclosure)
	if ValidateResponse(request, duplicate) == nil {
		t.Fatal("duplicate enclosure ID accepted")
	}
	unsafe := response
	unsafe.Enclosures[0].Filename = "../manifesto.txt"
	if ValidateResponse(request, unsafe) == nil {
		t.Fatal("unsafe filename accepted")
	}
	oversized := response
	oversized.Enclosures[0].Size = (2 << 20) + 1
	oversized.Enclosures[0].Content = nil
	if ValidateResponse(request, oversized) == nil {
		t.Fatal("oversized enclosure accepted")
	}
	empty := NewBrowserSession(&fakeTransport{}, "test")
	if _, err := empty.prepareEnclosures(reused, request.URI, journey); !isMissingEnclosure(err) {
		t.Fatalf("missing cached enclosure did not fail safely: %v", err)
	}
}

func TestMultipartMIMEEnclosureDecoding(t *testing.T) {
	enclosure := enclosureFixture()
	response := MailWebResponse{MailWeb: "0.5", RequestID: "01J00000000000000000000000", Status: 200, Enclosures: []Enclosure{{ID: enclosure.ID, Filename: enclosure.Filename, MediaType: enclosure.MediaType, Size: enclosure.Size, Digest: enclosure.Digest}}, Document: &MailWebDocument{Title: "MIME", Body: []Node{}}}
	payload, _ := json.Marshal(response)
	raw := "From: browse@demo.local\r\nTo: postbox@client.local\r\nSubject: response\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=letter\r\n\r\n--letter\r\nContent-Type: application/mailweb+json\r\n\r\n" + string(payload) + "\r\n--letter\r\nContent-Type: text/plain\r\nContent-ID: <manifesto>\r\nContent-Transfer-Encoding: base64\r\n\r\nZW5jbG9zZWQgY29ycmVzcG9uZGVuY2U=\r\n--letter--\r\n"
	decoded, err := parseMailWebResponse([]byte(raw))
	if err != nil || !bytes.Equal(decoded.Enclosures[0].Content, enclosure.Content) {
		t.Fatalf("MIME enclosure failed: %#v %v", decoded, err)
	}
}

func TestHTTPTransportPreservesLogicalEnclosure(t *testing.T) {
	enclosure := enclosureFixture()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var incoming MailWebRequest
		_ = json.NewDecoder(request.Body).Decode(&incoming)
		_ = json.NewEncoder(writer).Encode(MailWebResponse{MailWeb: incoming.MailWeb, RequestID: incoming.ID, Status: 200, Enclosures: []Enclosure{enclosure}, Document: &MailWebDocument{Title: "HTTP", Body: []Node{}}})
	}))
	defer server.Close()
	request, _ := NewRequest("mailweb://demo.local/")
	response, err := NewHTTPTransport(server.URL).Exchange(context.Background(), request)
	if err != nil || !bytes.Equal(response.Enclosures[0].Content, enclosure.Content) {
		t.Fatalf("HTTP enclosure round trip failed: %v", err)
	}
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
