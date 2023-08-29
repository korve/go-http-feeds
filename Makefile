all: subscribe

subscribe:
	go build -o bin/httpfeed-subscribe cmd/http-subscribe/main.go
