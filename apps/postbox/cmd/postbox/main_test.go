package main

import (
	"bytes"
	"context"
	"io/fs"
	"strings"
	"testing"
)

type fakeTransport struct {
	requests []MailWebRequest
}

func (transport *fakeTransport) Exchange(_ context.Context, request MailWebRequest) (MailWebResponse, error) {
	transport.requests = append(transport.requests, request)
	path := strings.TrimPrefix(request.URI, "mailweb://demo.local")
	if path == "/" {
		return MailWebResponse{MailWeb: "0.1", RequestID: request.ID, Status: 200, Document: &MailWebDocument{
			Title: "Home", Body: []Node{{Type: "heading", Level: 1, Text: "Hello"}, {Type: "link", Label: "About", Href: "/about"}},
		}}, nil
	}
	return MailWebResponse{MailWeb: "0.1", RequestID: request.ID, Status: 200, Document: &MailWebDocument{
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
