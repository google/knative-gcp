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

package adapter

//import (
//	"context"
//	"encoding/json"
//	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/pubsubutil/fakepubsub"
//	"io/ioutil"
//	"net/http"
//	"net/http/httptest"
//	"testing"
//
//	"cloud.google.com/go/pubsub"
//	v1alpha1 "github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/events/v1alpha1"
//	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/kncloudevents"
//	cloudevents "github.com/cloudevents/sdk-go"
//	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
//	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
//	"go.uber.org/zap"
//)
//
//func TestReceiveMessage_ServeHTTP(t *testing.T) {
//	testCases := map[string]struct {
//		sink  func(http.ResponseWriter, *http.Request)
//		acked bool
//	}{
//		"happy": {
//			sink:  sinkAccepted,
//			acked: true,
//		},
//		"rejected": {
//			sink:  sinkRejected,
//			acked: false,
//		},
//	}
//	for n, tc := range testCases {
//		t.Run(n, func(t *testing.T) {
//			h := &fakeHandler{
//				handler: tc.sink,
//			}
//			sinkServer := httptest.NewServer(h)
//			defer sinkServer.Close()
//
//			fakePubSubClient := fakepubsub.Client{
//				Data: fakepubsub.ClientData{
//					TopicData: fakepubsub.TopicData{
//						Exists: true,
//					},
//				},
//			}
//
//			a := &Adapter{
//				SinkURI: sinkServer.URL,
//				inbound: func() client.Client {
//					tOpts := []cepubsub.Option{
//						cepubsub.WithBinaryEncoding(),
//						cepubsub.WithClient(fakePubSubClient),
//						cepubsub.WithProjectID(a.ProjectID),
//						cepubsub.WithTopicID(a.TopicID),
//						cepubsub.WithSubscriptionID(a.SubscriptionID),
//					}
//
//					// Make a pubsub transport for the CloudEvents client.
//					t, err := cepubsub.New(context.Background(), tOpts...)
//					if err != nil {
//						return nil, err
//					}
//
//					// Use the transport to make a new CloudEvents client.
//					c, err := cloudevents.NewClient(t,
//						cloudevents.WithUUIDs(),
//						cloudevents.WithTimeNow(),
//						cloudevents.WithConverterFn(a.convert),
//					)
//
//					if err != nil {
//						return nil, err
//					}
//					return c, nil
//				}(),
//				outbound: func() client.Client {
//					c, _ := kncloudevents.NewDefaultClient(sinkServer.URL)
//					return c
//				}(),
//			}
//
//			data, err := json.Marshal(map[string]string{"key": "value"})
//			if err != nil {
//				t.Errorf("unexpected error, %v", err)
//			}
//
//			m := &PubSubMockMessage{
//				MockID: "ABC",
//				M: &pubsub.Message{
//					ID:   "ABC",
//					Data: data,
//				},
//			}
//			a.receiveMessage(context.TODO(), m)
//
//			if tc.acked && m.Nacked {
//				t.Errorf("expected message to be acked, but was not.")
//			}
//
//			if !tc.acked && m.Acked {
//				t.Errorf("expected message to be nacked, but was not.")
//			}
//
//			if m.Acked == m.Nacked {
//				t.Errorf("Message has the same Ack and Nack status: %v", m.Acked)
//			}
//		})
//	}
//}
//
//type fakeHandler struct {
//	body   []byte
//	header http.Header
//
//	handler func(http.ResponseWriter, *http.Request)
//}
//
//func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
//	h.header = r.Header
//	body, err := ioutil.ReadAll(r.Body)
//	if err != nil {
//		http.Error(w, "can not read body", http.StatusBadRequest)
//		return
//	}
//	h.body = body
//
//	defer r.Body.Close()
//	h.handler(w, r)
//}
//
//func sinkAccepted(writer http.ResponseWriter, req *http.Request) {
//	writer.WriteHeader(http.StatusOK)
//}
//
//func sinkRejected(writer http.ResponseWriter, _ *http.Request) {
//	writer.WriteHeader(http.StatusRequestTimeout)
//}
