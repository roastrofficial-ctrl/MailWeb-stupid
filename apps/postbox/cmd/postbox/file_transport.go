package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileTransport is local Postbox IPC. A separately scheduled Postbox-side Host
// adapter may collect it and use PORTER; no remote application is invoked here.
type FileTransport struct {
	Root    string
	Timeout time.Duration
}

func NewFileTransport(root string, timeout time.Duration) *FileTransport {
	return &FileTransport{Root: root, Timeout: timeout}
}

func (transport *FileTransport) Exchange(ctx context.Context, request MailWebRequest) (MailWebResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return MailWebResponse{}, err
	}
	outgoing := filepath.Join(transport.Root, "outgoing")
	incoming := filepath.Join(transport.Root, "incoming")
	if err := os.MkdirAll(outgoing, 0777); err != nil {
		return MailWebResponse{}, err
	}
	if err := os.MkdirAll(incoming, 0777); err != nil {
		return MailWebResponse{}, err
	}
	target := filepath.Join(outgoing, request.ID+".json")
	temporary := target + ".tmp"
	if err := os.WriteFile(temporary, payload, 0666); err != nil {
		return MailWebResponse{}, err
	}
	if err := os.Rename(temporary, target); err != nil {
		return MailWebResponse{}, err
	}
	observeJourney(ctx, "transport.local.lodged", "MailWeb correspondence entered local Postbox IPC", map[string]string{"request_id": request.ID})

	responsePath := filepath.Join(incoming, request.ID+".json")
	deadline := time.NewTimer(transport.Timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return MailWebResponse{}, ctx.Err()
		case <-deadline.C:
			return MailWebResponse{}, fmt.Errorf("timed out after %s waiting for local correspondence", transport.Timeout)
		case <-ticker.C:
			value, err := os.ReadFile(responsePath)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return MailWebResponse{}, err
			}
			var response MailWebResponse
			if err := json.Unmarshal(value, &response); err != nil {
				return MailWebResponse{}, err
			}
			_ = os.Remove(responsePath)
			return response, nil
		}
	}
}
