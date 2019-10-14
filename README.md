# sloop - Kubernetes History Visualization

Sloop monitors Kubernetes, recording histories of events and resource state changes 
and providing visualizations to aid in debugging past events.  

Key features:

1. Allows you to find and inspect resources that no longer exist (example: discover what host the pod from the previous deployment was using).
1. Provides timeline displays that show rollouts of related resources in updates to Deployments, ReplicaSets, and StatefulSets.
1. Helps debug transient and intermittent errors.
1. Allows you to see changes over time in a Kubernetes application.
1. Is a self-contained service with no dependencies on distributed storage.

## Screenshots

![Screenshot1](other/screenshot1.png?raw=true "Screenshot 1")

## Architecture Overview

![Architecture](other/architecture.png?raw=true "Architecture")

## Install

Sloop can be installed using any of these options:

### Helm Chart

Users can install sloop by using helm chart now, for instructions refer [helm readme](helm/sloop/README.md)

### Precompiled Binaries

_DockerHub images coming soon._

### Build from Source

Building Sloop from source needs a working Go environment
with [version 1.13 or greater installed](https://golang.org/doc/install).

Clone the sloop repository and build using `make`:

```sh
mkdir -p $GOPATH/src/github.com/salesforce
cd $GOPATH/src/github.com/salesforce
git clone https://github.com/salesforce/sloop.git
cd sloop
make
$GOPATH/bin/sloop
```

When complete, you should have a running Sloop version accessing the current context from your kubeConfig. Just point your browser at http://localhost:8080/

Other makefile targets:

* *docker*: Builds a Docker image.
* *cover*: Runs unit tests with code coverage.
* *generate*: Updates genny templates for typed table classes.
* *protobuf*: Generates protobuf code-gen.

### Local Docker Run

To run from Docker you need to host mount your kubeconfig:

```shell script
make docker
docker run --rm -it -p 8080:8080 -v ~/.kube/:/kube/ -e KUBECONFIG=/kube/config sloop
```

In this mode, data is written to a memory-backed volume and is discarded after each run. To preserve the data, you can host-mount /data with something like `-v /data/:/some_path_on_host/`

## Contributing

Refer to [CONTRIBUTING.md](CONTRIBUTING.md)<br>
[Join our Slack](https://join.slack.com/t/sfdc-sloop/shared_invite/enQtNzgzOTcxOTUzMjM4LTgzMWJhZGU5ZTRjM2ZmNGE5ZDI1OTg4MTc2NDI4ZjAxMTZhMzE1MTc3NTI0M2M0MTcwY2JjOTQ0ZTIzMWU5OWQ)

## License

BSD 3-Clause
