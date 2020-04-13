package config_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/broker/config"
)

func TestWatcherNoConfig(t *testing.T) {
	t.Parallel()
	in := make(chan *config.TargetsConfig)
	defer close(in)
	watcher := config.NewTargetsWatcher(in)
	targets := watcher.Targets()
	if diff := cmp.Diff(config.TargetsConfig{}, *targets.Config); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestWatcherWithConfig(t *testing.T) {
	t.Parallel()
	in := make(chan *config.TargetsConfig)
	defer close(in)
	watcher := config.NewTargetsWatcher(in)
	want := new(config.TargetsConfig)
	in <- want
	targets := watcher.Targets()
	if targets.Config != want {
		t.Errorf("targets.Config: want %v, got %v", want, targets.Config)
	}
}

func TestWatcherNotUpdated(t *testing.T) {
	t.Parallel()
	in := make(chan *config.TargetsConfig)
	defer close(in)
	watcher := config.NewTargetsWatcher(in)
	targets := watcher.Targets()
	select {
	case <-targets.Updated:
		t.Error("targets.Updated signalled spuriously")
	case <-time.After(1 * time.Millisecond):
	}
}

func TestWatcherUpdated(t *testing.T) {
	t.Parallel()
	in := make(chan *config.TargetsConfig)
	defer close(in)
	watcher := config.NewTargetsWatcher(in)
	targets := watcher.Targets()
	in <- new(config.TargetsConfig)
	select {
	case <-targets.Updated:
	case <-time.After(1 * time.Millisecond):
		t.Error("targets.Updated not signalled")
	}
}

func TestWatcherFanout(t *testing.T) {
	t.Parallel()
	in := make(chan *config.TargetsConfig)
	defer close(in)
	watcher := config.NewTargetsWatcher(in)
	targets1 := watcher.Targets()
	targets2 := watcher.Targets()
	want := new(config.TargetsConfig)
	in <- want
	<-targets1.Updated
	targets1 = watcher.Targets()
	<-targets2.Updated
	targets2 = watcher.Targets()
	if targets1.Config != want {
		t.Errorf("targets1.Config: want %v, got %v", want, targets1.Config)
	}
	if targets2.Config != want {
		t.Errorf("targets2.Config: want %v, got %v", want, targets2.Config)
	}
}
