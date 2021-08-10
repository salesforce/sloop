.PHONY:all run linux docker generate tidy protobuf cover docker-push

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

goreleaser:
	 @if [ ! -f "$(GOPATH)/bin/goreleaser" ];then \
   		curl -sfL https://install.goreleaser.com/github.com/goreleaser/goreleaser.sh | sh -s -- -b "$(GOPATH)/bin/"; \
   	 fi

docker-snapshot: goreleaser
	$(GOPATH)/bin/goreleaser release --snapshot --rm-dist

docker: goreleaser
	$(GOPATH)/bin/goreleaser release --rm-dist --skip-publish

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

release:
	@if [ ! -z "$(GITHUB_TOKEN)" ];then \
    	curl -sfL https://git.io/goreleaser | sh -s -- release --rm-dist;\
  	else \
  	  	curl -sfL https://git.io/goreleaser | sh -s -- release --rm-dist --skip-publish && \
        docker push sloopimage/sloop;\
  	fi