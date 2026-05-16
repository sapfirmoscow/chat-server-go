package ws

import "encoding/json"

//mb (typing, read in future)
type EventType string

const (
	EventMessageNew EventType = "message.new"
)

type Event struct {
	Type EventType   `json:"type"`
	Data interface{} `json:"data"`
}

func Marshal(t EventType, data interface{}) ([]byte, error) {
	return json.Marshal(Event{Type: t, Data: data})
}
