package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderCompletesMessageRoundTrip(t *testing.T) {
	publisher := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request requestMessage
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(responseMessage{
			MailWeb: "0.1", RequestID: request.ID, Status: 200,
			Document: document{Title: "Hello", Body: []node{{Type: "heading", Level: 1, Text: "Hello"}}},
		})
	}))
	defer publisher.Close()

	s := &server{publisherURL: publisher.URL, client: publisher.Client()}
	recorder := httptest.NewRecorder()
	s.render(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "<h1>Hello</h1>") {
		t.Fatalf("unexpected response: %d %q", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-MailWeb-Request-ID") == "" {
		t.Fatal("request correlation ID was not exposed")
	}
}

func TestRenderDocumentEscapesTextAndRejectsScriptURIs(t *testing.T) {
	output, err := renderDocument(document{Title: "<title>", Body: []node{{Type: "paragraph", Text: "<script>"}}})
	if err != nil || strings.Contains(string(output), "<script>") {
		t.Fatal("plain text was not escaped")
	}
	_, err = renderDocument(document{Title: "Unsafe", Body: []node{{Type: "link", Label: "Run", Href: "javascript:alert(1)"}}})
	if err == nil {
		t.Fatal("executable URI was accepted")
	}
}
