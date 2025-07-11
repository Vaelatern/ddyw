.PHONY: build build-raw

build-raw: ddyw

ddyw: internal/* cmd/ddyw
	go build ./cmd/ddyw
