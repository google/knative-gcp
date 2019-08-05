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

package e2e

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	yaml "github.com/jcrossley3/manifestival/pkg/manifestival"
	"k8s.io/client-go/dynamic"
)

func NewInstaller(dc dynamic.Interface, config map[string]string, paths ...string) *Installer {
	if len(paths) == 0 || (len(paths) == 1 && paths[0] == "") {
		// default to ko path:
		paths[0] = "/var/run/ko/install"
	}

	for i, p := range paths {
		paths[i] = ParseTemplates(p, config)
	}
	path := strings.Join(paths, ",")

	manifest, err := yaml.NewYamlManifest(path, true, dc)
	if err != nil {
		panic(err)
	}
	return &Installer{dc: dc, manifest: manifest}
}

// YamlPathsOptionFunc allows for bulk mutation of the yaml paths.
type YamlPathsOptionFunc func([]string) []string

// EndToEndConfigYaml assembles yaml from the local config directory.
// Note: `config` dir is assumed to be relative to the caller path.
func EndToEndConfigYaml(paths []string, options ...YamlPathsOptionFunc) []string {
	// TODO: this could be smarter in the future, look for e2e or something.
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)

	yamls := make([]string, 0, len(paths))
	for _, path := range paths {
		yamls = append(yamls, fmt.Sprintf("%s/config/%s/", dir, path))
	}

	for _, o := range options {
		yamls = o(yamls)
	}

	return yamls
}

func ParseTemplates(path string, config map[string]string) string {
	dir, err := ioutil.TempDir("", "processed_yaml")
	if err != nil {
		panic(err)
	}

	err = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), "yaml") {
			t, err := template.ParseFiles(path)
			if err != nil {
				return err
			}
			tmpfile, err := ioutil.TempFile(dir, strings.Replace(info.Name(), ".yaml", "-*.yaml", 1))
			if err != nil {
				log.Fatal(err)
			}
			err = t.Execute(tmpfile, config)
			if err != nil {
				log.Print("execute: ", err)
				return err
			}
			_ = tmpfile.Close()
		}
		return nil
	})
	log.Print("new files in ", dir)
	if err != nil {
		panic(err)
	}
	return dir
}

type Installer struct {
	dc dynamic.Interface

	manifest yaml.Manifest
}

func (r *Installer) Do(verb string) error {
	switch strings.ToLower(verb) {
	case "create", "setup", "install", "apply", "start":
		return r.manifest.ApplyAll()
	case "delete", "teardown", "uninstall", "unapply", "stop":
		return r.manifest.DeleteAll()
	default:
		return fmt.Errorf("unknown verb: %s", verb)
	}
}
