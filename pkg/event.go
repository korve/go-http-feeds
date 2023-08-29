package pkg

import (
	"time"
)

// Event represents a CloudEvent. See  https://github.com/cloudevents/spec
type Event struct {
	SpecVersion     string                 `json:"specversion"`               // The currently supported CloudEvents specification version.
	ID              string                 `json:"id"`                        // A unique value for this event.
	Type            string                 `json:"type"`                      // The type of the event.
	Source          string                 `json:"source"`                    // The source system that created the event. Should be a URI of the system.
	Time            time.Time              `json:"time"`                      // The event addition timestamp. ISO 8601 UTC date and time format.
	Subject         string                 `json:"subject"`                   // Key to identify the business object.
	Method          string                 `json:"method,omitempty"`          // The HTTP equivalent method type that the feed item performs on the subject. Defaults to PUT.
	DataContentType string                 `json:"datacontenttype,omitempty"` // Defaults to application/json.
	Data            map[string]interface{} `json:"data,omitempty"`            // The payload of the item.
}
