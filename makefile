# Makefile

.PHONY: all clean run build

build:
	go build -o bin/gstream .

run: build
	./bin/gstream
testp:
	go run cmd/testpublish/main.go



clean:
	rm -f bin/gstream
