## Build

Sloop uses GitHub to manage contributions. You'll need to [sign up for a GitHub account](https://docs.github.com/en/github/getting-started-with-github/signing-up-for-a-new-github-account) if you don't have one.

## Steps to Contribute

1. [Pick an issue you are interested in working on](https://github.com/salesforce/sloop/issues).
   It’s a good idea to comment on an Issue that you want to help with and ask some questions before doing any major coding work

2. Fork the repository: Look for the “Fork” button in the top right corner of [salesforce/sloop repo](https://github.com/salesforce/sloop)

3. Clone the fork: Click on the green “clone or download” button,you can choose “clone with HTTPS.” Or, you can use SSH, which is compatible with 2-factor authentication, but just [some more work to set up](https://docs.github.com/en/github/authenticating-to-github/connecting-to-github-with-ssh).
   Go to the folder where you want to save the repository, type `git clone that-https-url-you-copied`

4. Add a remote pointing to the original repository so you can sync changes: `git remote add upstream git@github.com:salesforce/sloop.git`

5. Make a git branch: If you type git status you will see the branch name that you’re on, called master. In most projects, master is a special place where the most stable, reviewed, up-to-date code is. So, you’ll need to make your own branch and switch to that branch:
   `git checkout -b name-for-your-branch`

6. Make some commits:

When you have some code that you want to keep, you should save it in git by creating a commit. Here’s how:

`git status` shows you the files you changed

`git add path-to-your-file` allows you to pre-select the files you want to save

`git status` again to make sure you added the files you want to keep

`git commit -m "some message here #123"` groups your changes together into a commit. The message should be short, describe the work that you did, and include the issue number that you are working on.

`git push origin name-for-your-branch` to save your work online

7. Open a Pull Request: To create one, go to your fork of the project, click on the Pull Requests tab, and click the big green “New Pull Request” button. After you choose your branch,
   click the green “Create Pull Request” button. It will be super helpful if you can write a sentence or two summarizing the work you did and include a link to the Issue you were working on.
   If you are working on a new issue, please create one under Issues tab

8. Expect changes and be patient: Next up, a maintainer or contributor will review your code. Once your work is approved, it will be merged in!
   Congratulations and thank you for giving back to the open source community :)

## Pull Request Checklist

1. Design

2. Functionality

3. Tests

4. Naming

5. Comments

## Versioning & Dependency Management

### Go Version

The Go Version required in Sloop is primarily influenced by two factors.

Firstly, the Go project supports 2 versions at a time for bug fixes and
vulnerability patching. We target the lowest supported version to maximise
compatibility with users systems.

Secondly, the Kubernetes client library used by Sloop for interacting with
Kubernetes clusters defines its own required Go version.

Sloop uses whichever of these two versions is the most recent.

### Kubernetes client-go

Sloop uses the [client-go](https://github.com/kubernetes/client-go) library for
interacting with Kubernetes clusters.

The Kubernetes project has its own release cadence and maintains support for
each release for a period of time. This support includes fixing bugs and
patching vulnerabilities.

The Sloop project aims to only support Kubernetes releases that are currently
maintained by the Kubernetes project. This is a sliding window of releases.

When a new Kubernetes version is released it may require updating the Go
version to support the new `client-go` version compatible with the Kubernetes
API.

It's possible that Sloop may continue to work for older end-of-life Kubernetes
releases but that is outside of the Sloop project goals.

### Go Modules

Sloop uses [go modules](https://golang.org/cmd/go/#hdr-Modules__module_versions__and_more).
This requires a working Go environment with the version defined in the
[go.mod](./go.mod) file or greater installed.

To add or update a new dependency:

1. use `go get` to pull in the new dependency
1. run `go mod tidy`

## Protobuf Schema Changes

When changing schema in pkg/sloop/store/typed/schema.proto you will need to do the following:

1. Install protobuf. On OSX you can do `brew install protobuf`
1. Grab protoc-gen-go with `go get -u github.com/golang/protobuf/protoc-gen-go`
1. Run this makefile target: `make protobuf`

## Changes to Generated Code

Sloop uses genny to code-gen typed table wrappers. Any changes to `pkg/sloop/store/typed/tabletemplate*.go` will need
to be followed with `go generate`. We have a Makefile target for this: `make generate`
