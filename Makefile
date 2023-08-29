all: subscribe

subscribe:
	go build -o bin/httpfeed-subscribe cmd/subscribe/main.go
