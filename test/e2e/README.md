# E2E Tests

Prow will run `./e2e-tests.sh`.

## Adding E2E Tests

E2E tests are tagged with `// +build e2e` but tagging a go file this way will
prevent the compiler from compiling the test code. To work around this, for the
SmokeTest we will have two files:

```shell
test/e2e
├── smoke.go
└── smoke_test.go
```

- `smoke_test.go` is the testing file entry point (tagged with e2e).
- `smoke.go` is the test implementation (not tagged with e2e).

For SmokeTest, we will try an experimental way to implement e2e tests, we will
use lightly templatized `yaml` files to create the testing setup. And then go to
observe the results.

SmokeTest uses `yaml` files in `config/smoke_test/*` like this:

```shell
test/e2e
├── config
│   └── smoke_test
│       └── smoke-test.yaml
├── smoke.go
└── smoke_test.go
```

The SmokeTest test reads in all of these yamls and passes them through a golang
template processor:

```golang
	yamls := fmt.Sprintf("%s/config/smoke_test/", filepath.Dir(filename))
	installer := NewInstaller(client.Dynamic, map[string]string{
		"namespace": client.Namespace,
	}, yamls)
```

The `map[string]string` above is the template parameters that will be used for
the `yaml` files.

The test then uses `installer` like you might think about using `kubectl` to
setup the test namespace:

```golang
	// Create the resources for the test.
	if err := installer.Do("create"); err != nil {
		t.Errorf("failed to create, %s", err)
	}
```

This is similar to `kubectl create -f ./config/smoke_test` after the templates
have been processed.

After this point, you can write tests and interact with the cluster and
resources like normal. SmokeTest uses a dynamic client, but you can add typed
clients if you wish.

## To run manually using `go test` and an existing cluster

```shell
go test --tags=e2e ./test/e2e/...
```

And count is supported too:

```shell
go test --tags=e2e ./test/e2e/... --count=3
```

If you want to run a specific test:

```shell
go test --tags=e2e ./test/e2e/... -run NameOfTest
```

For example, to run TestPullSubscription:

```shell
GOOGLE_APPLICATION_CREDENTIALS=<path to json creds file> \
GCP_PROJECT=<project name> \
  go test --tags=e2e ./test/e2e/... -run TestPullSubscription
```
