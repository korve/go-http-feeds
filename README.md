# go-http-feeds

This is an [HTTP feeds](https://www.http-feeds.org/) client written in Go. HTTP feeds is a minimal specification for polling events over HTTP.

## Usage

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/korve/go-http-feeds"
)

func main() {
	endpoint := "http://localhost:8080/feed" // the HTTP feed endpoint to poll
	lastEventId := ""                        // empty string to get all events or a specific event id to get events after that 
	pollDelay := 5 * time.Second             // delay between polls
	timeoutDuration := 10 * time.Second      // timeout query parameter value (see https://www.http-feeds.org/#data-model)

	events := make(chan httpfeeds.Event)
	ctx := context.Background()

	// subscribe to events in a separate goroutine 
	go func() {
		client := httpfeeds.NewClient(pollDelay, timeoutDuration)
		err = client.Subscribe(endpoint, lastEventId, events, ctx)
		if err != nil {
			// ... handle error
		}
	}()

	// infinite loop to process events. The loop will receive new events when they are available.
	for event := range events {
		// access event metadata
		fmt.Println(event)

		// access event data
		fmt.Printf("SKU: %s\n", event.Data["sku"])
	}
}
```

## License

Apache 2.0, see [LICENSE](LICENSE)
