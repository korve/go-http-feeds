package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/korve/go-http-feeds/pkg"
	"net/url"
	"os"
	"time"
)

var pollDelay int
var timeout int
var lastEventId string
var verbose bool

func printUsage() {
	fmt.Printf("Usage: %s [options] <endpoint>\n", os.Args[0])
	fmt.Printf("  <endpoint>: HTTP feed endpoint to subscribe to\n")

	flag.PrintDefaults()
}

// Subscribes to a HTTP Feed using the Client and subscription types.
func main() {
	flag.IntVar(&pollDelay, "poll-delay", 5000, "Poll delay in milliseconds between each poll to the HTTP endpoint")
	flag.IntVar(&timeout, "timeout", 0, "timeout in milliseconds until the server must send a response")
	flag.StringVar(&lastEventId, "last-event-id", "", "Last event ID received by the client")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.Parse()

	endpoint := flag.Arg(0)
	if endpoint == "" {
		printUsage()
		os.Exit(1)
	}

	if _, err := url.Parse(endpoint); err != nil {
		panic("endpoint must be a valid URL")
	}

	var err error
	var pollDelayDuration time.Duration
	var timeoutDuration time.Duration

	if pollDelay > 0 {
		pollDelayDuration, err = time.ParseDuration(fmt.Sprintf("%dms", pollDelay))
		if err != nil {
			fmt.Printf("invalid poll delay: %v\n", err)
			os.Exit(1)
		}
	}
	if timeout > 0 {
		timeoutDuration, err = time.ParseDuration(fmt.Sprintf("%dms", timeout))
		if err != nil {
			fmt.Printf("invalid timeout: %v\n", err)
			os.Exit(1)
		}
	}

	if verbose {
		fmt.Printf("subscribing to:\n")
		fmt.Printf("endpoint: %s\n", endpoint)
		fmt.Printf("pollDelay: %s\n", pollDelayDuration)
		fmt.Printf("timeout: %s\n", timeoutDuration)
		fmt.Printf("lastEventId: %s\n", lastEventId)
	}

	events := make(chan pkg.Event)
	ctx := context.Background()

	go func() {
		client := pkg.NewClient(pkg.ClientOptions{
			PollDelay: pollDelayDuration,
			Timeout:   timeoutDuration,
		})
		err = client.Subscribe(endpoint, lastEventId, events, ctx)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			os.Exit(1)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			cause := context.Cause(ctx)
			if cause != nil && !errors.Is(cause, context.Canceled) {
				panic(cause)
			}
			return

		case e := <-events:
			fmt.Printf("%s\n", e.Data["sku"])
		}
	}
}
