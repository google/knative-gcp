The CRD generation tool generates CRD YAMLs by using common snippets 
located in [hack/crdgeneration/snippets](./snippets). 

### Execution

It is run during `hack/update-codegen.sh` and verified before submission by `hack/verify-codegen.sh`.

If you wish to run it manually, use the following command from the repo root directory:

```shell
REPO_ROOT_DIR=$(pwd) hack/crdgeneration/generate_crds.sh
```

### How it works

The tool reads in all the files that end in `.yaml` in the 
[hack/crdgeneration/snippets](./snippets) directory. It then goes through the
[config](../../config) directory and for each file named `FOO.gofmt.yaml` it 
uses [Go text/template](https://golang.org/pkg/text/template/) to produce the
output file `FOO.yaml`. The one function that has been custom defined is
`replace_with`, which has the following signature:

```golang
func replace_with(numSpaces int, snippet string)
```

* `numSpaces` is the number of leading spaces to add to each line inserted. It is
needed to match the surrounding YAML indentation (as YAML is left space 
sensitive). 
* `snippet` is the name of the snippet to inject. It is the snippet file's 
name minus `.yaml`. So to inject the snippet file `foo.yaml`, the value
of `snippet` should be `foo`.


For example:

```yaml
spec:
  {{- replace_with 2 "foo" }}
  bar: baz
```

Would use the file `foo.yaml` from the snippets directory and insert it with every
line preceded by two spaces.

### Reasoning

Keeping the OpenAPIV3Schema accurate for all the CRD YAMLs was very tedious and
error prone. Many of our CRDs share types. For example, most of our Sources have
`spec.pubSubSpec` and `status.pubSubStatus`. Rather than write the same schema in
each CRD's YAML, it is far easier and less error prone to write the schema for
those types once ([pubsub_spec.yaml](./snippets/pubsub_spec.yaml) and 
[pubsub_status.yaml](./snippets/pubsub_status.yaml)) and have all the CRD YAMLs
use those canonical versions. 
