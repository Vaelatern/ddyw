.PHONY: build build-raw

build-raw: ddyw

ddyw: internal/* cmd/ddyw
	go build ./cmd/ddyw

clean:
	rm -f ddyw c.out coverage.html

test:
	go test ./... -cover

coverage.html:  internal/* cmd/ddyw
	go test ./... -coverprofile=c.out
	go tool cover -html=c.out -o coverage.html

