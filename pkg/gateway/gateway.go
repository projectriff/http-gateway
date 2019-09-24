/*
 * Copyright 2019 The original author or authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gateway

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	sclient "github.com/projectriff/http-gateway/pkg/client"
	"k8s.io/apimachinery/pkg/types"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"

	alpha1 "github.com/projectriff/system/pkg/apis/streaming/v1alpha1"
)

const mimeTypeOctetStream = "application/octet-stream"

const leaseDuration = time.Duration(1 * time.Minute)

type Gateway struct {
	server        *http.Server
	k8sClient     client2.Reader
	streamClients map[alpha1.StreamAddress]*lease
	mutex         sync.Mutex
}

type lease struct {
	client *sclient.StreamClient
	expiry time.Time
}

func NewGateway(reader client2.Reader) *Gateway {
	g := Gateway{
		k8sClient:     reader,
		streamClients: make(map[alpha1.StreamAddress]*lease),
	}

	m := http.NewServeMux()
	m.HandleFunc("/", g.ingest)

	g.server = &http.Server{
		Addr:    ":8080",
		Handler: m,
	}
	return &g
}

func (g *Gateway) Run(stopCh <-chan struct{}) error {
	err := g.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	} else {
		<-stopCh
		return nil
	}
}

func (g *Gateway) ingest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Only POSTs are accepted"))
		return
	}
	parts := strings.Split(r.RequestURI[1:], "/")
	if len(parts) != 2 {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Request URI should be of the form /<namespace>/<stream-name>"))
		return
	}

	namespacedName := types.NamespacedName{Namespace: parts[0], Name: parts[1]}
	stream := alpha1.Stream{}
	if err := g.k8sClient.Get(context.Background(), namespacedName, &stream); err != nil {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(fmt.Sprintf("Stream %s not found", namespacedName)))
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mimeTypeOctetStream
	}

	client, err := g.lookupClient(&stream)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error locating sclient for stream"))
		return
	}
	headers := make(map[string]string) // TODO: Decide which http headers to copy over based eg on WL/BL rules
	if _, err := client.Publish(context.Background(), r.Body, nil, contentType, headers); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, "Error publishing to stream: %v", err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (g *Gateway) Shutdown(ctx context.Context) error {
	if err := g.server.Shutdown(ctx); err != nil {
		return err
	}
	var err error = nil
	for _, c := range g.streamClients {
		if e := c.client.Close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}

func (g *Gateway) lookupClient(stream *alpha1.Stream) (*sclient.StreamClient, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	defer g.purgeClients()

	if l, ok := g.streamClients[stream.Status.Address]; ok {
		l.expiry = time.Now().Add(leaseDuration)
		return l.client, nil
	}

	streamClient, err := sclient.NewStreamClient(stream.Status.Address.Gateway, stream.Status.Address.Topic, stream.Spec.ContentType)
	if err != nil {
		return nil, err
	}
	g.streamClients[stream.Status.Address] = &lease{client: streamClient, expiry: time.Now().Add(leaseDuration)}

	return streamClient, nil
}

func (g *Gateway) purgeClients() {
	now := time.Now()
	for gw, l := range g.streamClients {
		if now.After(l.expiry) {
			delete(g.streamClients, gw)
			if err := l.client.Close(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "error closing expired gRPC sclient: %v\n", err)
			}
		}
	}
}
