package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const DefaultPollDelay = 5 * time.Second
const DefaultRequestTimeout = 30 * time.Second

type Client struct {
	pollDelay      time.Duration
	timeout        time.Duration
	requestTimeout time.Duration
}

type ClientOptions struct {
	// pollDelay is the delay between each poll to the HTTP endpoint. Defaults to 5 seconds.
	PollDelay time.Duration

	// timeout is set, when long-polling should be used and is supported by the server. Max waiting time for long-polling, after which the server must send a response. A typical value is 5s.
	Timeout time.Duration

	// requestTimeout is the timeout for the polling HTTP request.
	// Defaults to 30 seconds. When the timeout is reached, the request will be retried and no error will be returned.
	// Warning: If using Timeout, the requestTimeout should be set to a value lower than Timeout, otherwise the client will run into an error.
	RequestTimeout time.Duration
}

type subscription struct {
	lastEventId string
}

// NewClient creates a new Client.
func NewClient(opts ClientOptions) *Client {
	pollDelay := opts.PollDelay
	if pollDelay == 0 {
		pollDelay = DefaultPollDelay
	}

	requestTimeout := opts.RequestTimeout
	if requestTimeout == 0 {
		requestTimeout = DefaultRequestTimeout
	}

	return &Client{
		pollDelay:      pollDelay,
		timeout:        opts.Timeout,
		requestTimeout: requestTimeout,
	}
}

// Subscribe subscribes to an HTTP Stream. Returns a channel that will receive the stream data.
// endpoint string - The HTTP endpoint to subscribe to.
// lastEventId string - The last event ID received by the client. Leave empty to start from the beginning.
// events chan Event - The channel that will receive the event stream data.
// ctx context.Context - The context that will be used to cancel the subscription.
func (c *Client) Subscribe(endpoint string, lastEventId string, events chan Event, ctx context.Context) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	s := subscription{
		lastEventId: lastEventId,
	}

	ctx = context.WithValue(ctx, "subscription", &s)

	return c.startSubscription(u, lastEventId, events, ctx)
}

func (c *Client) startSubscription(u *url.URL, lastEventId string, events chan Event, ctx context.Context) error {
	ticker := time.NewTicker(c.pollDelay)
	defer ticker.Stop()

	f := func() error {
		sub := getSubscription(ctx)
		if sub.lastEventId != "" {
			lastEventId = sub.lastEventId
		}

		e, err := c.fetchEvents(u.String(), lastEventId, ctx)
		if err != nil {
			return err
		}

		// Process the events right after fetching
		for _, event := range e {
			sub.lastEventId = event.ID
			events <- event
		}

		// If we're using simple polling and the response is empty, reset the ticker
		if c.timeout == 0 && len(e) == 0 {
			ticker.Reset(c.pollDelay)
		}

		return nil
	}

	// Initiate the first request immediately
	if err := f(); err != nil {
		ticker.Reset(c.pollDelay) // Reset ticker in case of an error
	}

	for {
		select {
		// cancelled
		case <-ctx.Done():
			fmt.Printf("status: cancelled\n")
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return nil

		case <-ticker.C:
			if err := f(); err != nil {
				ticker.Reset(c.pollDelay) // Reset ticker in case of an error
			}
		}
	}
}

func (c *Client) fetchEvents(endpoint, lastEventId string, ctx context.Context) ([]Event, error) {
	// Create GET request
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	query := u.Query()
	if lastEventId != "" {
		query.Set("lastEventId", lastEventId)
	} else {
		query.Set("lastEventId", "")
	}

	if c.timeout != 0 {
		query.Set("timeout", strconv.FormatInt(c.timeout.Milliseconds(), 10))
	}

	u.RawQuery = query.Encode()

	// create timeout context
	if c.requestTimeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	// Send GET request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check if status code is OK
	if resp.StatusCode != http.StatusOK {
		if resp.ContentLength > 0 {
			// read body
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("got error response from server. status: %s, body: %s", resp.Status, b)
		}

		return nil, fmt.Errorf("got error response from server. status: %s", resp.Status)
	}

	var events []Event
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&events); err != nil {
		return nil, err
	}

	return events, nil
}

// getSubscription returns the subscription from the context.
func getSubscription(ctx context.Context) *subscription {
	return ctx.Value("subscription").(*subscription)
}
