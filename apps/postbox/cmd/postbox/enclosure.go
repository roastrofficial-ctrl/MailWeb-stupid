package main

import (
	"errors"
	"sort"
	"time"
)

type EnclosureFile struct {
	ID           string    `json:"id"`
	Filename     string    `json:"filename"`
	MediaType    string    `json:"media_type"`
	Size         int64     `json:"size"`
	Digest       string    `json:"digest"`
	ReceivedAt   time.Time `json:"received_at"`
	ReceivedWith []string  `json:"received_with"`
	Content      []byte    `json:"-"`
}
type MissingEnclosureError struct{ ID, Digest string }

func (err MissingEnclosureError) Error() string {
	return "missing enclosure " + err.ID + " " + err.Digest
}

func (session *BrowserSession) prepareEnclosures(response MailWebResponse, uri string, journey *journeyRecorder) (MailWebResponse, error) {
	if len(response.Enclosures) > 0 {
		journey.add("enclosure.discovered", "enclosures declared by correspondence", map[string]string{"count": jsonNumber(len(response.Enclosures))})
	}
	for index, enclosure := range response.Enclosures {
		if len(enclosure.Content) > 0 {
			journey.add("enclosure.hash_validated", "enclosure digest validated", map[string]string{"id": enclosure.ID, "digest": enclosure.Digest, "bytes": jsonNumber(len(enclosure.Content))})
			session.mu.Lock()
			file, exists := session.enclosures[enclosure.Digest]
			if !exists && len(session.enclosures) >= 64 {
				session.mu.Unlock()
				return response, errors.New("enclosure store limit reached")
			}
			if !exists {
				file = EnclosureFile{ID: enclosure.ID, Filename: enclosure.Filename, MediaType: enclosure.MediaType, Size: enclosure.Size, Digest: enclosure.Digest, ReceivedAt: time.Now().UTC(), Content: append([]byte(nil), enclosure.Content...)}
				journey.add("enclosure.filed", "enclosure filed in Postbox", map[string]string{"id": enclosure.ID, "digest": enclosure.Digest})
			}
			if !contains(file.ReceivedWith, uri) {
				file.ReceivedWith = append(file.ReceivedWith, uri)
				sort.Strings(file.ReceivedWith)
			}
			session.enclosures[enclosure.Digest] = file
			session.mu.Unlock()
		} else {
			session.mu.Lock()
			_, exists := session.enclosures[enclosure.Digest]
			session.mu.Unlock()
			if !exists {
				return response, MissingEnclosureError{ID: enclosure.ID, Digest: enclosure.Digest}
			}
			journey.add("enclosure.found", "enclosure found in Postbox", map[string]string{"id": enclosure.ID, "digest": enclosure.Digest})
		}
		response.Enclosures[index].Content = nil
	}
	return response, nil
}
func (session *BrowserSession) Enclosure(digest string) (EnclosureFile, bool) {
	session.mu.Lock()
	defer session.mu.Unlock()
	file, ok := session.enclosures[digest]
	return file, ok
}
func isMissingEnclosure(err error) bool {
	var target MissingEnclosureError
	return errors.As(err, &target)
}
