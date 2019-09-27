package main

import (
	"log"
	"net/http"
	"net/http/httputil"
)

type MessageDumper struct{}

func (md *MessageDumper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if reqBytes, err := httputil.DumpRequest(r, true); err == nil {
		log.Printf("Message Dumper received a message: %+v", string(reqBytes))
		w.Write(reqBytes)
	} else {
		log.Printf("Error dumping the request: %+v :: %+v", err, r)
	}
}

func main() {
	http.ListenAndServe(":8080", &MessageDumper{})
}
