package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type config struct {
	// Seeders is the number of seeders expected to
	// report metrics.
	Seeders int64 `envconfig:"SEEDERS"`
	// Actors is the number of actors expected to
	// report metrics.
	Actors int64 `envconfig:"ACTORS"`
}

func main() {
	var env config
	if err := envconfig.Process("", &env); err != nil {
		log.Fatalf("Failed to process env config: %v", err)
	}

	h := &handler{}
	srv := &http.Server{
		Addr:    ":8080",
		Handler: h,
	}

	go func() {
		for {
			time.Sleep(5 * time.Second)
			if env.Seeders == atomic.LoadInt64(&h.seeders) && env.Actors == atomic.LoadInt64(&h.actors) {
				tlog := fmt.Sprintf(
					`{"successSent": %d, "failureSent": %d, "received": %d}`,
					atomic.LoadInt64(&h.successSentCount),
					atomic.LoadInt64(&h.failureSentCount),
					atomic.LoadInt64(&h.receivedCount),
				)

				if err := ioutil.WriteFile("/dev/termination-log", []byte(tlog), 0644); err != nil {
					log.Fatalf("Failed to write termination log: %v", err)
				}

				srv.Shutdown(context.Background())
				break
			} else {
				seederDiff := env.Seeders - atomic.LoadInt64(&h.seeders)
				actorDiff := env.Actors - atomic.LoadInt64(&h.actors)
				log.Printf("Waiting for %d seeders and %d actors...\n", seederDiff, actorDiff)
			}
		}
	}()

	log.Println("Starting aggregator server...")
	log.Println(srv.ListenAndServe())
}

type handler struct {
	seeders int64
	actors  int64

	successSentCount int64
	failureSentCount int64
	receivedCount    int64
}

func (s *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Printf("Received request with headers: %v\n", req.Header)

	role := req.Header.Get("role")
	if role == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if role == "actor" {
		c := req.Header.Get("count")
		delta, err := strconv.ParseInt(c, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		atomic.AddInt64(&s.receivedCount, delta)
		atomic.AddInt64(&s.actors, 1)
	} else {
		sc := req.Header.Get("success")
		scDelta, err := strconv.ParseInt(sc, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		fc := req.Header.Get("failure")
		fcDelta, err := strconv.ParseInt(fc, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		atomic.AddInt64(&s.successSentCount, scDelta)
		atomic.AddInt64(&s.failureSentCount, fcDelta)
		atomic.AddInt64(&s.seeders, 1)
	}

	w.WriteHeader(http.StatusOK)
}
