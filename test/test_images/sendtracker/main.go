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
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
)

// Compare the ranges in sendResults and receiveResults.
// Create error strings for any entries that are successful in sendResults, but not receiveResults.
// Create warning strings for any entries that are in receiveResults, but not successfull in sendResults.
// Append an error string if sendError is not nil.
func compareLogs(sendResults rangeResultArr, receiveResults rangeReceivedArr, sendError error) (errorLogs, warningLogs string) {
	successSendRangeCount := 0
	if sendError != nil {
		errorLogs = sendError.Error()
	}
	for _, senderI := range sendResults {
		if senderI.success {
			successSendRangeCount++
			found := false
			for _, receiverI := range receiveResults {
				if receiverI.minID <= senderI.minID && receiverI.maxID >= senderI.maxID {
					found = true
					if receiverI.minID != senderI.minID || receiverI.maxID != senderI.maxID {
						warningLogs = fmt.Sprintf("%sExtra elements in received range for sender range %+v\n", warningLogs, senderI)
					}
				}
			}
			if !found {
				errorLogs = fmt.Sprintf("%sElements not received from %+v\n", errorLogs, senderI)
			}

		}
	}
	if successSendRangeCount == 0 {
		errorLogs = fmt.Sprintf("%sNo successful sends\n", errorLogs)
	}
	if successSendRangeCount < len(receiveResults) {
		warningLogs = fmt.Sprintf("%sExtra range in received range set\n", warningLogs)
	}
	return errorLogs, warningLogs
}

func main() {
	sinkURI := os.Getenv("K_SINK")
	if len(sinkURI) == 0 {
		fmt.Printf("No SINK URI specified\n")
		os.Exit(1)
	}
	delayMSString := os.Getenv("DELAY_MS")
	delayMS, err := strconv.Atoi(delayMSString)
	if err != nil || delayMS < 0 {
		fmt.Printf("Invalid delay millisecond (%s)", delayMSString)
		os.Exit(1)
	}
	postStopWaitSecsString := os.Getenv("POST_STOP_SECS")
	postStopWaitSecs, err := strconv.Atoi(postStopWaitSecsString)
	if err != nil || postStopWaitSecs < 1 {
		fmt.Printf("Invalid post stop wait seconds (%s)", postStopWaitSecsString)
		os.Exit(1)
	}
	fmt.Printf("Opening client to %s\n", sinkURI)
	httpOpts := []cehttp.Option{
		cloudevents.WithTarget(sinkURI),
	}
	transport, err := cloudevents.NewHTTP(httpOpts...)
	if err != nil {
		fmt.Printf("failed to create transport: %v\n", err)
		os.Exit(1)
	}

	ceClient, err := cloudevents.NewClient(transport)
	if err != nil {
		fmt.Printf("Unable to create ceClient: %s ", err)
		os.Exit(1)
	}
	fmt.Printf("Client created to %s\n", sinkURI)

	bint, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		fmt.Printf("Error creating stracker random source id: %v", bint)
		os.Exit(1)
	}
	source := fmt.Sprintf("stracker_%s", bint)
	// Start the receive goroutine
	receiver := StartReceiver(source, time.Duration(postStopWaitSecs)*time.Second)
	sendPacer := makeSender(ceClient, receiver, source, time.Duration(delayMS))

	http.HandleFunc("/stop", sendPacer.handleStop)
	http.HandleFunc("/start", sendPacer.handleStart)
	http.HandleFunc("/ready", sendPacer.handleReady)
	http.HandleFunc("/resultsready", sendPacer.handleResultsReady)
	http.HandleFunc("/results", sendPacer.handleResults)
	go http.ListenAndServe(":8070", nil)

	// Run the send path synchronously
	sendError := sendPacer.Run()
	if sendError != nil && sendPacer.getState() < SendStateFinishing {
		fmt.Printf("Sender saw an error and never made it to finishing: %v\n", sendError)
	} else {
		fmt.Printf("Waiting for receiver to drain\n")
		<-receiver.fullyDone
		// Get the events the receiver saw
		results, dups := receiver.GetResults()

		// Compare with the events the sender sent
		errorString, warningString := compareLogs(sendPacer.results, results, sendError)

		var resultSummary string
		// If there were no errors seen, it was a success (warnings don't fail this)
		if len(errorString) == 0 {
			resultSummary = "success\n"
		} else {
			resultSummary = "failure\n"
		}
		sendPacer.resultSummary = fmt.Sprintf("%s%s%sAll send intervals = %s\nAll receive intervals = %s\nDuplicates seen %d\n", resultSummary, errorString, warningString, sendPacer.results, results, dups)
		fmt.Printf("%s", sendPacer.resultSummary)
		// Allow the result to be retrieved via /results
		sendPacer.setState(SendStateDone)
	}

	// loop forever
	for {
		time.Sleep(time.Minute)
	}
}
