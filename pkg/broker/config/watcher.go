package config

// TargetsWatcher is an interface for watching a dynamically updated TargetsConfig.
type TargetsWatcher interface {
	// Returns the latest TargetsConfig value in
	// Targets.Config. Targets.Updated contains a channel which will be
	// closed when there is new config value to be read.
	Targets() Targets
}

// Targets contains a single Config as well as an Updated channel which will be
// closed after Config becomes stale.
type Targets struct {
	Config  *TargetsConfig
	Updated chan struct{}
}

type watcher <-chan Targets

func (w watcher) Targets() Targets {
	return <-w
}

// NewTargetsWatcher creates a new TargetsWatcher which watches for updates on
// in until in is closed
func NewTargetsWatcher(in <-chan *TargetsConfig) TargetsWatcher {
	w := make(chan Targets)
	go watch(in, w)
	return watcher(w)
}

func watch(in <-chan *TargetsConfig, out chan<- Targets) {
	defer close(out)
	targets := Targets{
		Config:  new(TargetsConfig),
		Updated: make(chan struct{}),
	}
	for {
		select {
		case out <- targets:
		case c, ok := <-in:
			if !ok {
				return
			}
			close(targets.Updated)
			targets = Targets{
				Config:  c,
				Updated: make(chan struct{}),
			}
		}
	}
}
