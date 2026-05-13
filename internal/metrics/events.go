package metrics

import "time"

type Event struct {
	Name      string
	Timestamp time.Time
	Labels    map[string]string
}

type EventSink interface {
	Sink
	PublishEvent(event Event)
	Events() []Event
}
