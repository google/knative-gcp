/*
Copyright 2019 Google LLC

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

package e2etest

import (
	"fmt"
	"strings"

	yaml "github.com/jcrossley3/manifestival/pkg/manifestival"
	"k8s.io/client-go/dynamic"
)

func NewInstaller(ns string, dc dynamic.Interface, paths ...string) *Installer {
	var path string
	if len(paths) == 0 || (len(path) == 1 && paths[0] == "") {
		// default to ko path:
		path = "/var/run/ko/install"
	} else {
		path = strings.Join(paths, ",")
	}

	manifest, err := yaml.NewYamlManifest(path, true, dc)
	if err != nil {
		panic(err)
	}
	return &Installer{ns: ns, dc: dc, manifest: manifest}
}

type Installer struct {
	ns string
	dc dynamic.Interface

	manifest yaml.Manifest
}

func (r *Installer) Do(verb string) error {

	// TODO: might need to take the paths and apply a namespace.

	switch strings.ToLower(verb) {
	case "create", "setup", "install", "apply", "start":
		return r.manifest.ApplyAll()
	case "delete", "teardown", "uninstall", "unapply", "stop":
		return r.manifest.DeleteAll()
	default:
		return fmt.Errorf("unknown verb: %s", verb)
	}
}
