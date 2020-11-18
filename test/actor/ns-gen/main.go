/*
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const nsTemplate = `apiVersion: v1
kind: Namespace
metadata:
  name: {{.namespace}}
`

const brTemplate = `apiVersion: eventing.knative.dev/v1beta1
kind: Broker
metadata:
  name: testbroker
  namespace: {{.namespace}}
  annotations:
    "eventing.knative.dev/broker.class": "{{.brclass}}"
`

const envTemplate = `        - name: {{.envname}}
          value: {{.envvalue}}
`
const trTemplate = `apiVersion: v1
kind: Service
metadata:
  name: actor-{{.index}}
  namespace: {{.namespace}}
spec:
  selector:
    app: actor-{{.index}}
  ports:
    - protocol: TCP
      port: 80
      targetPort: 8080
      name: http
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: actor-{{.index}}
  namespace: {{.namespace}}
  labels:
    app: actor-{{.index}}
spec:
  replicas: {{.replicas}}
  selector:
    matchLabels:
      app: actor-{{.index}}
  template:
    metadata:
      labels:
        app: actor-{{.index}}
    spec:
      containers:
      - name: actor
        image: ko://github.com/google/knative-gcp/test/actor/actor
        resources:
          requests:
            memory: "400Mi"
          limits:
            memory: "400Mi"
        ports:
        - containerPort: 8080
        env:
        - name: AGGREGATOR_ADDR
          value: http://aggregator.{{.namespace}}.svc.cluster.local
{{.envs}}
---
apiVersion: eventing.knative.dev/v1beta1
kind: Trigger
metadata:
  name: trigger-actor-{{.index}}
  namespace: {{.namespace}}
spec:
  broker: testbroker
  filter:
    attributes:
      type: seed
  subscriber:
    ref:
     apiVersion: v1
     kind: Service
     name: actor-{{.index}}
---
`
const seederTemplate = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: seeder
  namespace: {{.namespace}}
  labels:
    app: seeder
spec:
  replicas: {{.replicas}}
  selector:
    matchLabels:
      app: seeder
  template:
    metadata:
      labels:
        app: seeder
    spec:
      containers:
      - name: seeder
        image: ko://github.com/google/knative-gcp/test/actor/seeder
        env:
        - name: TARGET
          value: http://default-brokercell-ingress.cloud-run-events.svc.cluster.local/{{.namespace}}/testbroker
        - name: INTERVAL
          value: {{.interval}}
        - name: SIZE
          value: "{{.size}}"
        - name: CONCURRENCY
          value: "{{.concurrency}}"
        - name: ELAPSE
          value: {{.elapse}}
        - name: AGGREGATOR_ADDR
          value: http://aggregator.{{.namespace}}.svc.cluster.local
`

const aggTemplate = `apiVersion: v1
kind: Service
metadata:
  name: aggregator
  namespace: {{.namespace}}
spec:
  selector:
    role: aggregator
  ports:
    - name: http
      port: 80
      targetPort: 8080
      protocol: TCP
---
apiVersion: batch/v1
kind: Job
metadata:
  name: aggregator
  namespace: {{.namespace}}
  labels:
    role: aggregator
spec:
  completions: 1
  parallelism: 1
  backoffLimit: 0
  template:
    metadata:
      labels:
        role: aggregator
    spec:
      restartPolicy: Never
      containers:
      - name: aggregator
        image: ko://github.com/google/knative-gcp/test/actor/aggregator
        env:
          - name: SEEDERS
            value: "{{.seeders}}"
          - name: ACTORS
            value: "{{.actors}}"
        ports:
          - name: http
            containerPort: 8080
`

var (
	output         = flag.String("output", "", "Output path")
	ns             = flag.String("ns", "default", "Namesapce")
	triggerCount   = flag.Int("triggers", 100, "The number of triggers to create")
	errRate        = flag.Int("err-rate", 0, "Fail requests with the given error rate")
	respDelay      = flag.String("resp-delay", "", "Delay for all requests")
	seedInternal   = flag.String("interval", "1s", "Seed interval")
	payloadSize    = flag.Int64("payload-size", 100, "The size of the event payload")
	brClass        = flag.String("br-class", "googlecloud", "The broker class")
	actorReplicas  = flag.Int("actors", 1, "The number of actor replicas")
	actorMaxConn   = flag.Int("max-conn", 0, "The max conns an actor pod accepts concurrently")
	seederReplicas = flag.Int("seeders", 1, "The number of seeder replicas")
	seederConn     = flag.Int("seeders-conn", 1, "The seeder concurrency")
	elapse         = flag.String("elapse", "15m", "The seeding elapse")
)

func main() {
	flag.Parse()

	namespace := strings.ReplaceAll(nsTemplate, "{{.namespace}}", *ns)

	br := strings.ReplaceAll(brTemplate, "{{.namespace}}", *ns)
	br = strings.ReplaceAll(br, "{{.brclass}}", *brClass)

	envs := ""
	if *errRate > 0 {
		env1 := strings.ReplaceAll(envTemplate, "{{.envname}}", "ERR_HOSTS")
		env1 = strings.ReplaceAll(env1, "{{.envvalue}}", `"*"`)
		envs += env1

		env2 := strings.ReplaceAll(envTemplate, "{{.envname}}", "ERR_RATE")
		env2 = strings.ReplaceAll(env2, "{{.envvalue}}", fmt.Sprintf(`"%d"`, *errRate))
		envs += env2
	}
	if *respDelay != "" {
		env1 := strings.ReplaceAll(envTemplate, "{{.envname}}", "DELAY_HOSTS")
		env1 = strings.ReplaceAll(env1, "{{.envvalue}}", `"*"`)
		envs += env1

		env2 := strings.ReplaceAll(envTemplate, "{{.envname}}", "DELAY")
		env2 = strings.ReplaceAll(env2, "{{.envvalue}}", *respDelay)
		envs += env2
	}
	if *actorMaxConn > 0 {
		env := strings.ReplaceAll(envTemplate, "{{.envname}}", "MAX_CONN")
		env = strings.ReplaceAll(env, "{{.envvalue}}", fmt.Sprintf(`"%d"`, *actorMaxConn))
		envs += env
	}

	triggers := ""
	for i := 0; i < *triggerCount; i++ {
		tr := strings.ReplaceAll(trTemplate, "{{.namespace}}", *ns)
		tr = strings.ReplaceAll(tr, "{{.index}}", strconv.Itoa(i))
		tr = strings.ReplaceAll(tr, "{{.replicas}}", strconv.Itoa(*actorReplicas))
		tr = strings.ReplaceAll(tr, "{{.envs}}", envs)
		triggers += tr
	}

	seeder := strings.ReplaceAll(seederTemplate, "{{.namespace}}", *ns)
	seeder = strings.ReplaceAll(seeder, "{{.interval}}", *seedInternal)
	seeder = strings.ReplaceAll(seeder, "{{.size}}", fmt.Sprintf("%d", *payloadSize))
	seeder = strings.ReplaceAll(seeder, "{{.replicas}}", strconv.Itoa(*seederReplicas))
	seeder = strings.ReplaceAll(seeder, "{{.concurrency}}", fmt.Sprintf("%d", *seederConn))
	if *elapse != "" {
		seeder = strings.ReplaceAll(seeder, "{{.elapse}}", *elapse)
	}

	aggregator := strings.ReplaceAll(aggTemplate, "{{.namespace}}", *ns)
	aggregator = strings.ReplaceAll(aggregator, "{{.seeders}}", fmt.Sprintf("%d", *seederReplicas))
	aggregator = strings.ReplaceAll(aggregator, "{{.actors}}", fmt.Sprintf("%d", (*actorReplicas)*(*triggerCount)))

	if err := ioutil.WriteFile(filepath.Join(*output, "00-namespace.yaml"), []byte(namespace), 0644); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	if err := ioutil.WriteFile(filepath.Join(*output, "01-broker.yaml"), []byte(br), 0644); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	if err := ioutil.WriteFile(filepath.Join(*output, "02-aggregator.yaml"), []byte(aggregator), 0644); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	if err := ioutil.WriteFile(filepath.Join(*output, "03-triggers.yaml"), []byte(triggers), 0644); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	if err := ioutil.WriteFile(filepath.Join(*output, "04-seeder.yaml"), []byte(seeder), 0644); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
