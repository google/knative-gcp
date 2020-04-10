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

package volume

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/protobuf/proto"
	"github.com/google/knative-gcp/pkg/broker/config"
)

const (
	defaultPath = "/var/run/cloud-run-events/broker/targets"
)

// targets watches a target config file and config updates to targetChan
type targets struct {
	path           string
	targetChan     chan<- *config.TargetsConfig
	realConfigFile string
	watcher        *fsnotify.Watcher
}

// NewTargetsFromFile initializes the targets config from a file and returns the target chan.
func NewTargetsFromFile(opts ...Option) (<-chan *config.TargetsConfig, error) {
	ch := make(chan *config.TargetsConfig)
	t := &targets{
		path:       defaultPath,
		targetChan: ch,
	}

	for _, opt := range opts {
		opt(t)
	}

	if watcher, err := fsnotify.NewWatcher(); err != nil {
		return nil, err
	} else {
		t.watcher = watcher
	}
	go t.watch()
	return ch, nil
}

func (t *targets) watch() {
	defer close(t.targetChan)
	defer t.watcher.Close()
	configFile := filepath.Clean(t.path)
	configDir, _ := filepath.Split(t.path)
	t.realConfigFile, _ = filepath.EvalSymlinks(t.path)
	t.watcher.Add(configDir)

	// send initial config
	if config, err := t.sync(); err != nil {
		log.Printf("error syncing config: %v\n", err)
	} else {
		t.sendConfig(config)
	}

	for {
		select {
		case event, ok := <-t.watcher.Events:
			if !ok {
				// 'Events' channel is closed.
				return
			}
			if t.shouldSync(&event) {
				if config, err := t.sync(); err != nil {
					log.Printf("error syncing config: %v\n", err)
				} else {
					t.sendConfig(config)
				}
			} else {
				if filepath.Clean(event.Name) == configFile && event.Op&fsnotify.Remove != 0 {
					return
				}
			}

		case err, ok := <-t.watcher.Errors:
			if ok {
				log.Printf("watcher error: %v\n", err)
			}
			return
		}
	}
}

func (t *targets) sendConfig(config *config.TargetsConfig) {
	for {
		select {
		case t.targetChan <- config:
			return
		case event := <-t.watcher.Events:
			if t.shouldSync(&event) {
				if newConfig, err := t.sync(); err != nil {
					log.Printf("error syncing config: %v\n", err)
				} else {
					config = newConfig
				}
			}
		}
	}

}

func (t *targets) shouldSync(event *fsnotify.Event) bool {
	configFile := filepath.Clean(t.path)
	currentConfigFile, _ := filepath.EvalSymlinks(t.path)
	// Re-sync if the file was updated/created or
	// if the real file was replaced.
	const writeOrCreateMask = fsnotify.Write | fsnotify.Create

	fileChanged := filepath.Clean(event.Name) == configFile && event.Op&writeOrCreateMask != 0
	fileReplaced := currentConfigFile != "" && currentConfigFile != t.realConfigFile
	if fileReplaced {
		t.realConfigFile = currentConfigFile
	}
	return fileChanged || fileReplaced
}

func (t *targets) watchWith(watcher *fsnotify.Watcher) {

	go func() {
		defer watcher.Close()
	}()
}

func (t *targets) sync() (*config.TargetsConfig, error) {
	b, err := t.readFile()
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var val config.TargetsConfig
	if err := proto.Unmarshal(b, &val); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file: %w", err)
	}

	return &val, nil
}

func (t *targets) readFile() ([]byte, error) {
	return ioutil.ReadFile(t.path)
}
