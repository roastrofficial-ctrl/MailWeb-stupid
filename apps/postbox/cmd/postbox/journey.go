package main

import (
	"context"
	"sync"
	"time"
)

type JourneyEvent struct {
	Timestamp time.Time         `json:"timestamp"`
	Type      string            `json:"type"`
	Label     string            `json:"label"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type Journey struct {
	ID              string           `json:"id"`
	URI             string           `json:"uri"`
	Method          string           `json:"method"`
	Protocol        string           `json:"protocol"`
	Transport       string           `json:"transport"`
	Delivery        string           `json:"delivery"`
	StartedAt       time.Time        `json:"started_at"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
	Outcome         string           `json:"outcome"`
	RoundTripMS     int64            `json:"round_trip_ms"`
	NetworkWaitMS   int64            `json:"network_wait_ms"`
	OriginalJourney string           `json:"original_journey_id,omitempty"`
	Events          []JourneyEvent   `json:"events"`
	Request         *MailWebRequest  `json:"request,omitempty"`
	Response        *MailWebResponse `json:"response,omitempty"`
}

type journeyRecorder struct {
	mu      sync.Mutex
	journey Journey
}

func newJourney(uri, method, transport string) *journeyRecorder {
	now := time.Now().UTC()
	recorder := &journeyRecorder{journey: Journey{ID: "mw_" + newID(), URI: uri, Method: method, Protocol: "0.6", Transport: transport, Delivery: "live correspondence", StartedAt: now, Outcome: "active", Events: []JourneyEvent{}}}
	recorder.addAt(now, "navigation.started", "navigation requested", map[string]string{"uri": uri, "method": method})
	return recorder
}

func (recorder *journeyRecorder) add(eventType, label string, metadata map[string]string) {
	recorder.addAt(time.Now().UTC(), eventType, label, metadata)
}

func (recorder *journeyRecorder) addAt(timestamp time.Time, eventType, label string, metadata map[string]string) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if count := len(recorder.journey.Events); count > 0 && timestamp.Before(recorder.journey.Events[count-1].Timestamp) {
		timestamp = recorder.journey.Events[count-1].Timestamp
	}
	recorder.journey.Events = append(recorder.journey.Events, JourneyEvent{Timestamp: timestamp, Type: eventType, Label: label, Metadata: metadata})
}

func (recorder *journeyRecorder) finish(outcome, delivery string, request *MailWebRequest, response *MailWebResponse, roundTrip, networkWait int64) Journey {
	now := time.Now().UTC()
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.journey.CompletedAt = &now
	recorder.journey.Outcome = outcome
	recorder.journey.Delivery = delivery
	recorder.journey.RoundTripMS = roundTrip
	recorder.journey.NetworkWaitMS = networkWait
	recorder.journey.Request = request
	recorder.journey.Response = response
	return recorder.journey
}

type journeyObserverKey struct{}

func withJourneyObserver(ctx context.Context, recorder *journeyRecorder) context.Context {
	return context.WithValue(ctx, journeyObserverKey{}, func(eventType, label string, metadata map[string]string) { recorder.add(eventType, label, metadata) })
}

func observeJourney(ctx context.Context, eventType, label string, metadata map[string]string) {
	if observer, ok := ctx.Value(journeyObserverKey{}).(func(string, string, map[string]string)); ok {
		observer(eventType, label, metadata)
	}
}
