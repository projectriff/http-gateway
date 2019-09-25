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

package main

import (
	"fmt"
	"time"

	"github.com/projectriff/http-gateway/pkg/gateway"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	streamingv1alpha1 "github.com/projectriff/system/pkg/apis/streaming/v1alpha1"
)

var (
	scheme     = runtime.NewScheme()
	syncPeriod = 10 * time.Hour
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = streamingv1alpha1.AddToScheme(scheme)
}

// +kubebuilder:rbac:groups=streaming.projectriff.io,resources=streams,verbs=get;watch

func main() {
	config := controllerruntime.GetConfigOrDie()
	mapper, err := apiutil.NewDiscoveryRESTMapper(config)
	if err != nil {
		panic(err)
	}

	cache, err := cache.New(config, cache.Options{Scheme: scheme, Mapper: mapper, Resync: &syncPeriod})
	if err != nil {
		panic(err)
	}

	stopCh := make(<-chan struct{})
	go func() {
		if err := cache.Start(stopCh); err != nil {
			panic(err)
		}
	}()
	fmt.Println("Waiting for caches to synch")
	cache.WaitForCacheSync(stopCh)
	fmt.Println("Caches synched, starting http server")

	gw := gateway.NewGateway(cache)
	if err := gw.Run(stopCh); err != nil {
		panic(err)
	}
}
