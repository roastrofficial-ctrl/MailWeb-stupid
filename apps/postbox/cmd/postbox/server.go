package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
)

//go:embed ui/*
var uiFiles embed.FS

type uiServer struct {
	session *BrowserSession
}

func RunWebUI(listenAddress string, session *BrowserSession) error {
	host, _, err := net.SplitHostPort(listenAddress)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		if host != "0.0.0.0" || envOr("MAILWEB_CONTAINER_LISTEN", "") != "true" {
			return errors.New("Postbox UI may only listen on the local loopback interface")
		}
	}
	assets, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		return err
	}
	server := &uiServer{session: session}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", server.state)
	mux.HandleFunc("POST /api/navigate", server.navigate)
	mux.HandleFunc("POST /api/form", server.submitForm)
	mux.HandleFunc("POST /api/back", server.back)
	mux.HandleFunc("POST /api/forward", server.forward)
	mux.HandleFunc("POST /api/reload", server.reload)
	mux.HandleFunc("GET /api/enclosures/{digest}", server.enclosure)
	mux.Handle("GET /", http.FileServerFS(assets))

	handler := securityHeaders(mux)
	log.Printf("Postbox UI listening on http://%s", listenAddress)
	return http.ListenAndServe(listenAddress, handler)
}

func (server *uiServer) enclosure(writer http.ResponseWriter, request *http.Request) {
	digest := "sha256:" + request.PathValue("digest")
	file, ok := server.session.Enclosure(digest)
	if !ok {
		http.NotFound(writer, request)
		return
	}
	writer.Header().Set("Content-Type", file.MediaType)
	writer.Header().Set("Content-Length", fmt.Sprint(len(file.Content)))
	writer.Header().Set("Content-Disposition", `inline; filename="`+strings.ReplaceAll(file.Filename, `"`, "")+`"`)
	writer.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = writer.Write(file.Content)
}

func (server *uiServer) submitForm(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Method string            `json:"method"`
		Action string            `json:"action"`
		Values map[string]string `json:"values"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeState(writer, server.session.Snapshot(), fmt.Errorf("invalid form submission: %w", err))
		return
	}
	state, err := server.session.SubmitForm(request.Context(), input.Method, input.Action, input.Values)
	writeState(writer, state, err)
}

func (server *uiServer) state(writer http.ResponseWriter, _ *http.Request) {
	writeState(writer, server.session.Snapshot(), nil)
}

func (server *uiServer) navigate(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		URI       string `json:"uri"`
		Reference string `json:"reference"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeState(writer, server.session.Snapshot(), fmt.Errorf("invalid navigation request: %w", err))
		return
	}
	var state BrowserState
	var err error
	if input.Reference != "" {
		state, err = server.session.NavigateReference(request.Context(), input.Reference)
	} else {
		state, err = server.session.Navigate(request.Context(), strings.TrimSpace(input.URI))
	}
	writeState(writer, state, err)
}

func (server *uiServer) back(writer http.ResponseWriter, request *http.Request) {
	state, err := server.session.Back(request.Context())
	writeState(writer, state, err)
}

func (server *uiServer) forward(writer http.ResponseWriter, request *http.Request) {
	state, err := server.session.Forward(request.Context())
	writeState(writer, state, err)
}

func (server *uiServer) reload(writer http.ResponseWriter, request *http.Request) {
	state, err := server.session.Reload(request.Context())
	writeState(writer, state, err)
}

func writeState(writer http.ResponseWriter, state BrowserState, err error) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	status := http.StatusOK
	response := struct {
		State BrowserState `json:"state"`
		Error string       `json:"error,omitempty"`
	}{State: state}
	if err != nil {
		status = http.StatusBadGateway
		response.Error = err.Error()
	}
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(response)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' http: https:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(writer, request)
	})
}
