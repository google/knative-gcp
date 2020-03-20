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

// Targets implementas config.ReadOnlyTargets with data
// loaded from a file.
// It also watches the file for any changes and will automatically
// refresh the in memory cache.
type Targets struct {
	config.BaseTargets
	path string
}

// Option is the option to load targets.
type Option func(*Targets)

// WithPath is the option to load targets from the given path.
func WithPath(path string) Option {
	return func(t *Targets) {
		t.path = path
	}
}

// NewTargetsFromFile initializes the targets config from a file.
func NewTargetsFromFile(opts ...Option) (config.ReadOnlyTargets, error) {
	t := &Targets{
		BaseTargets: config.BaseTargets{},
		path:        defaultPath,
	}

	for _, opt := range opts {
		opt(t)
	}

	if err := t.sync(); err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	initialized := make(chan struct{}, 1)
	go t.watchWith(watcher, initialized)
	// Block until we know for sure the events handler is properly set up.
	<-initialized
	return t, nil
}

func (t *Targets) watchWith(watcher *fsnotify.Watcher, initialized chan struct{}) {
	defer watcher.Close()
	done := make(chan struct{}, 1)
	configFile := filepath.Clean(t.path)
	configDir, _ := filepath.Split(t.path)
	realConfigFile, _ := filepath.EvalSymlinks(t.path)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					// 'Events' channel is closed.
					done <- struct{}{}
					return
				}
				currentConfigFile, _ := filepath.EvalSymlinks(t.path)

				// Re-sync if the file was updated/created or
				// if the real file was replaced.
				const writeOrCreateMask = fsnotify.Write | fsnotify.Create
				if (filepath.Clean(event.Name) == configFile &&
					event.Op&writeOrCreateMask != 0) ||
					(currentConfigFile != "" && currentConfigFile != realConfigFile) {
					realConfigFile = currentConfigFile
					if err := t.sync(); err != nil {
						log.Printf("error syncing config: %v\n", err)
					}
				} else if filepath.Clean(event.Name) == configFile &&
					event.Op&fsnotify.Remove != 0 {
					done <- struct{}{}
					return
				}

			case err, ok := <-watcher.Errors:
				if ok {
					log.Printf("watcher error: %v\n", err)
				}
				done <- struct{}{}
				return
			}
		}
	}()

	watcher.Add(configDir)
	initialized <- struct{}{}
	<-done
}

func (t *Targets) sync() error {
	b, err := t.readFile()
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg config.TargetsConfig
	if err := proto.Unmarshal(b, &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config file: %w", err)
	}

	t.Internal.Store(&cfg)
	return nil
}

func (t *Targets) readFile() ([]byte, error) {
	return ioutil.ReadFile(t.path)
}
