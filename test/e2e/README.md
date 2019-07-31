# E2E Tests

Prow will run `./e2e-tests.sh`. 

## To run manually using `go test` and an existing cluster 

```shell
go test --tags=e2e ./test/e2e/...
```

And count is supported too:

```shell
go test --tags=e2e ./test/e2e/... --count=3
```