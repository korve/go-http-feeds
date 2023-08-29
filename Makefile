all: subscribe

subscribe:
	go build -o dist/httpfeed-subscribe main.go
