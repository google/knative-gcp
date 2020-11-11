# Development

This doc explains how to setup a development environment so you can get started
contributing to `Knative-GCP`. Also take a look at:

- [The pull request workflow](https://www.knative.dev/contributing/contributing/#pull-requests)
- [How to add and run e2e tests](./test/e2e/README.md)
- [Iterating](#iterating)
- [Format check](#format-check)
- [Additional development documentation](./docs/development/README.md)

## Getting started

1. [Create and checkout a repo fork](#checkout-your-fork)

1. [Installing Knative-GCP](./README.md/#installing-knative-GCP)

### Checkout your fork

The Go tools require that you clone the repository to the
`src/github.com/google/knative-gcp` directory in your
[`GOPATH`](https://github.com/golang/go/wiki/SettingGOPATH).

To check out this repository:

1. Create your own
   [fork of this repo](https://help.github.com/articles/fork-a-repo/)
1. Clone it to your machine:

```shell
mkdir -p ${GOPATH}/src/github.com/google
cd ${GOPATH}/src/github.com/google
git clone git@github.com:${YOUR_GITHUB_USERNAME}/knative-gcp.git
cd knative-gcp
git remote add upstream https://github.com/google/knative-gcp.git
git remote set-url --push upstream no_push
```

_Adding the `upstream` remote sets you up nicely for regularly
[syncing your fork](https://help.github.com/articles/syncing-a-fork/)._

Once you reach this point you are ready to do a full build and deploy as
follows.

## Iterating

As you make changes to the code-base, there are two cases to be aware of(If you
run under macOS, you may need to upgrade your bash version on macOS since macOS
comes with bash version 3 which is quite limiting and lack features need to run
the following two scripts):

- **If you change a type definition ([pkg/apis/](./pkg/apis/.)),** then you must
  run [`./hack/update-codegen.sh`](./hack/update-codegen.sh).
- **If you change a package's deps** (including adding external dep), then you
  must run [`./hack/update-deps.sh`](./hack/update-deps.sh).

These are both idempotent, and we expect that running these at `HEAD` to have no
diffs.

Once the codegen and dependency information is correct, redeploying the
controller is simply:

```shell
ko apply -f config/500-controller.yaml
```

Or you can [clean it up completely](#clean-up) and start again.

## Format Check

The CI/CD runs format check, please run these commands before you submit a code
change to make sure the format follows the standard: For macOS:

```bash
find . -name '*.go' \! -name wire_gen.go \! -name '*.pb.go' -exec go run golang.org/x/tools/cmd/goimports -w {} \+ -o \( -path ./vendor -o -path ./third_party \) -prune
find . -name '*.go' -type f \! -name '*.pb.go' -exec gofmt -s -w {} \+ -o \( -path './vendor' -o -path './third_party' \) -prune
```

For Linux:

```bash
find -name '*.go' \! -name wire_gen.go \! -name '*.pb.go' -exec go run golang.org/x/tools/cmd/goimports -w {} \+ -o \( -path ./vendor -o -path ./third_party \) -prune
find -name '*.go' -type f \! -name '*.pb.go' -exec gofmt -s -w {} \+ -o \( -path './vendor' -o -path './third_party' \) -prune
```

## E2E Tests

Running E2E tests as you make changes to the code-base is pretty simple. See
[E2E test docs](./test/e2e/README.md).

## Clean up

You can delete `Knative-GCP` with:

```shell
ko delete -f config/
```
