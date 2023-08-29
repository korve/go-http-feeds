package pkg

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClient_fetchEvents(t *testing.T) {
	// 1. Set up a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastEventId := r.URL.Query().Get("lastEventId")
		if lastEventId == "error" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		fmt.Fprintln(w, `[{"id":"1"},{"id":"2"}]`)
	}))
	defer ts.Close()

	client := NewClient(ClientOptions{PollDelay: 10 * time.Millisecond})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 2. Test for successful response
	events, err := client.fetchEvents(ts.URL, "", ctx)
	assert.NoError(t, err)
	assert.Len(t, events, 2)
	assert.Equal(t, "1", events[0].ID)
	assert.Equal(t, "2", events[1].ID)
}

func TestClient_fetchEvents_setLastEventIdQueryParameter(t *testing.T) {
	// 1. Set up a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastEventId := r.URL.Query().Get("lastEventId")
		assert.Equal(t, lastEventId, "1")

		fmt.Fprintln(w, `[{"id":"2"}]`)
	}))
	defer ts.Close()

	client := NewClient(ClientOptions{PollDelay: 10 * time.Millisecond})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 2. Test for successful response
	events, err := client.fetchEvents(ts.URL, "1", ctx)
	assert.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "2", events[0].ID)
}

func TestClient_fetchEvents_setTimeoutQueryParameter(t *testing.T) {
	var timeoutQueryValue string

	// 1. Set up a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timeoutQueryValue = r.URL.Query().Get("timeout")
		fmt.Fprintln(w, `[{"id":"2"}]`)
	}))
	defer ts.Close()

	client := NewClient(ClientOptions{
		Timeout: 50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 2. Test for successful response
	events, err := client.fetchEvents(ts.URL, "1", ctx)
	assert.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "2", events[0].ID)

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(t, "50", timeoutQueryValue)
	}, 1*time.Second, 10*time.Millisecond)
}

func TestClient_fetchEvents_requestTimeout(t *testing.T) {
	// 1. Set up a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer ts.Close()

	client := NewClient(ClientOptions{
		PollDelay:      10 * time.Millisecond,
		RequestTimeout: 50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 2. Test for successful response
	_, err := client.fetchEvents(ts.URL, "", ctx)
	assert.Error(t, err)
	assert.Truef(t, errors.Is(err, context.DeadlineExceeded), "expected error to be DeadlineExceeded, got %v", err)
}

func TestClient_Subscribe_SimplePolling(t *testing.T) {
	var lastEventIdQueryValue string

	// 1. Setup a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastEventIdQueryValue = r.URL.Query().Get("lastEventId")
		fmt.Fprintln(w, `[{"id":"1"},{"id":"2"}]`)
	}))
	defer ts.Close()

	events := make(chan Event)
	client := NewClient(ClientOptions{PollDelay: 10 * time.Millisecond})

	var err error
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		err = client.Subscribe(ts.URL, "", events, ctx)
	}()

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(t, lastEventIdQueryValue, "")
		assert.NoError(c, err)
	}, 1*time.Second, 10*time.Millisecond)

	ev1 := <-events
	assert.Equal(t, "1", ev1.ID)

	ev2 := <-events
	assert.Equal(t, "2", ev2.ID)
}
func TestClient_Subscribe_LongPolling(t *testing.T) {
	// 1. Setup a test server to mimic long-polling behavior.
	// When a request comes in, it waits for a while before sending the response.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timeoutStr := r.URL.Query().Get("timeout")
		timeout, _ := time.ParseDuration(timeoutStr + "ms")

		lastEventId := r.URL.Query().Get("lastEventId")
		if lastEventId == "2" {
			time.Sleep(timeout - 90*time.Millisecond) // mimic a delay just under the long-poll timeout
			fmt.Fprintln(w, `[{"id":"3"}]`)
			return
		}

		if lastEventId == "" {
			time.Sleep(timeout - 90*time.Millisecond) // mimic a delay just under the long-poll timeout
			fmt.Fprintln(w, `[{"id":"1"},{"id":"2"}]`)
		}
	}))
	defer ts.Close()

	events := make(chan Event) // buffer to prevent blocking
	client := NewClient(ClientOptions{
		PollDelay: 10 * time.Millisecond,
		Timeout:   100 * time.Millisecond,
	})

	var err error
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		err = client.Subscribe(ts.URL, "", events, ctx)
	}()

	// Expect two events to be pushed immediately due to the test server's behavior
	ev1 := <-events
	ev2 := <-events
	// Given that the server waits almost the duration of the timeout,
	// the next event should be pulled right after the previous request.
	ev3 := <-events

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.NoError(c, err)
		assert.Equal(t, "1", ev1.ID)
		assert.Equal(t, "2", ev2.ID)
		assert.Equal(t, "3", ev3.ID)
	}, 500*time.Millisecond, 10*time.Millisecond)
}

func TestClient_Subscribe_WithUpdate(t *testing.T) {
	var lastEventIdQueryValue string

	// 1. Setup a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastEventIdQueryValue = r.URL.Query().Get("lastEventId")
		if lastEventIdQueryValue == "2" {
			fmt.Fprintln(w, `[{"id":"3"}]`)
			return
		}

		fmt.Fprintln(w, `[{"id":"1"},{"id":"2"}]`)
	}))
	defer ts.Close()

	events := make(chan Event)
	client := NewClient(ClientOptions{PollDelay: 10 * time.Millisecond})

	var err error
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		err = client.Subscribe(ts.URL, "", events, ctx)
	}()

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(t, lastEventIdQueryValue, "")
		assert.NoError(c, err)
	}, 1*time.Second, 100*time.Millisecond)

	ev1 := <-events
	assert.Equal(t, "1", ev1.ID)

	ev2 := <-events
	assert.Equal(t, "2", ev2.ID)

	// expect that 10ms after the last event, the client will poll the server again

	ev3 := <-events
	assert.Equal(t, "3", ev3.ID)
}
