.PHONY:perf perfasm

export GO111MODULE=on

all:
	go get ./pkg/...
	go fmt ./pkg/...
	go install ./pkg/...
	go test -cover ./pkg/...

run: 
	go install ./pkg/...
	$(GOPATH)/bin/sloop

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install -ldflags "-s" -installsuffix cgo -v ./pkg/...

docker: linux
	cp $(GOPATH)/bin/linux_amd64/sloop .
	docker build -t sloop .
	rm sloop

generate:
	go generate ./pkg/...

tidy:
	# Run tidy whenever go.mod is changed
	go mod tidy

protobuf:
	# Make sure you `brew install protobuf` first
	# go get -u github.com/golang/protobuf/protoc-gen-go
	protoc -I=./pkg/sloop/store/typed/ --go_out=./pkg/sloop/store/typed/ ./pkg/sloop/store/typed/schema.proto

cover:
	go test ./pkg/... -coverprofile=coverage.out
	go tool cover -html=coverage.out
