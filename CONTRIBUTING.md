## Build

Sloop uses GitHub to manager reviews of pull requests

## Steps to Contribute

ADD

## Pull Request Checklist

ADD

## Dependency Management

Sloop uses [go modules](https://golang.org/cmd/go/#hdr-Modules__module_versions__and_more).  
This requires a working Go environment with version 1.13 or greater installed.
It is suggested you set `export GO111MODULE=on`

To add or update a new dependency:

1. use `go get` to pull in the new dependency
1. run `go mod tidy`

## Protobuf Schema Changes

When changing schema in pkg/sloop/store/typed/schema.proto you will need to do the following:

1. Install protobuf.  On OSX you can do `brew install protobuf`
1. Grab protoc-gen-go with `go get -u github.com/golang/protobuf/protoc-gen-go`
1. Run this makefile target: `make protobuf`

## Changes to Generated Code

Sloop uses genny to code-gen typed table wrappers.  Any changes to `pkg/sloop/store/typed/tabletemplate*.go` will need 
to be followed with `go generate`.  We have a Makefile target for this: `make generate`

## Prometheus

Sloop uses prometheus to emit metrics, which is very helpful for performance debugging.  In the root of the repo is a prometheus config.

On OSX you can install prometheus with `brew install prometheus`.  Then simply start it from the sloop directory by running `prometheus`

Open your browser to http://localhost:9090.  
 
An example of a useful query is [rate(kubewatch_event_count[5m])](http://localhost:9090/graph?g0.range_input=1h&g0.expr=rate(kubewatch_event_count%5B1m%5D)&g0.tab=0)
