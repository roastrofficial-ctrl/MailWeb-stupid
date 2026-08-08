package main

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

type StationeryFile struct {
	ID         string          `json:"id"`
	Version    string          `json:"version"`
	Title      string          `json:"title"`
	ReceivedAt time.Time       `json:"received_at"`
	Document   MailWebDocument `json:"document"`
	Uses       []string        `json:"uses"`
}

type MissingStationeryError struct{ ID, Version string }

func (err MissingStationeryError) Error() string {
	return fmt.Sprintf("missing stationery %s %s", err.ID, err.Version)
}

func stationeryKey(id, version string) string { return id + "\x00" + version }

func (session *BrowserSession) prepareResponse(response MailWebResponse, uri string, journey *journeyRecorder) (MailWebResponse, error) {
	now := time.Now().UTC()
	for _, definition := range response.Templates {
		journey.add("template.enclosed", "new stationery enclosed", map[string]string{"id": definition.ID, "version": definition.Version})
		journey.add("template.validated", "stationery content identity validated", map[string]string{"id": definition.ID})
		key := stationeryKey(definition.ID, definition.Version)
		session.mu.Lock()
		file, exists := session.stationery[key]
		if !exists && len(session.stationery) >= 64 {
			session.mu.Unlock()
			return response, errors.New("stationery store limit reached")
		}
		if !exists {
			file = StationeryFile{ID: definition.ID, Version: definition.Version, Title: definition.Document.Title, ReceivedAt: now, Document: definition.Document}
		}
		session.stationery[key] = file
		session.mu.Unlock()
		if !exists {
			journey.add("stationery.filed", "stationery filed in Postbox", map[string]string{"id": definition.ID, "version": definition.Version})
		}
	}
	if response.Document.Template == "" {
		return response, nil
	}
	ref := response.Document
	journey.add("template.requested", "correspondence requested stationery", map[string]string{"id": ref.Template, "version": ref.TemplateVersion})
	key := stationeryKey(ref.Template, ref.TemplateVersion)
	session.mu.Lock()
	file, found := session.stationery[key]
	session.mu.Unlock()
	if !found {
		return response, MissingStationeryError{ID: ref.Template, Version: ref.TemplateVersion}
	}
	journey.add("stationery.found", "matching stationery found in Postbox", map[string]string{"id": ref.Template, "version": ref.TemplateVersion})
	knownSlots := map[string]bool{}
	for _, node := range file.Document.Body {
		if node.Type == "slot" {
			knownSlots[node.Name] = true
		}
	}
	for name := range ref.Slots {
		if !knownSlots[name] {
			return response, fmt.Errorf("response supplied unknown stationery slot %q", name)
		}
	}
	composed := file.Document
	composed.Body = make([]Node, 0, len(file.Document.Body))
	for _, node := range file.Document.Body {
		if node.Type != "slot" {
			composed.Body = append(composed.Body, node)
			continue
		}
		nodes := ref.Slots[node.Name]
		composed.Body = append(composed.Body, nodes...)
		journey.add("document.slot_populated", "stationery slot populated", map[string]string{"name": node.Name, "nodes": jsonNumber(len(nodes))})
	}
	if ref.Title != "" {
		composed.Title = ref.Title
	}
	if ref.Presentation != nil {
		composed.Presentation = mergePresentation(composed.Presentation, ref.Presentation)
	}
	response.Document = &composed
	journey.add("document.composed", "semantic document composed with stationery", map[string]string{"id": ref.Template, "version": ref.TemplateVersion})
	session.mu.Lock()
	file = session.stationery[key]
	if !contains(file.Uses, uri) {
		file.Uses = append(file.Uses, uri)
		sort.Strings(file.Uses)
		session.stationery[key] = file
	}
	session.mu.Unlock()
	return response, nil
}

func mergePresentation(base, override *Presentation) *Presentation {
	if base == nil && override == nil {
		return nil
	}
	var result Presentation
	if base != nil {
		result = *base
	}
	if override == nil {
		return &result
	}
	if override.Accent != "" {
		result.Accent = override.Accent
	}
	if override.Background != "" {
		result.Background = override.Background
	}
	if override.Foreground != "" {
		result.Foreground = override.Foreground
	}
	if override.Surface != "" {
		result.Surface = override.Surface
	}
	if override.Typeface != "" {
		result.Typeface = override.Typeface
	}
	if override.Density != "" {
		result.Density = override.Density
	}
	if override.Corners != "" {
		result.Corners = override.Corners
	}
	return &result
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
func isMissingStationery(err error) bool {
	var target MissingStationeryError
	return errors.As(err, &target)
}
