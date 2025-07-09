//
// Copyright 2022 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package k8sutil

import (
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/util"
)

func NewODLMCache(isolatedModeEnable bool, opts ctrl.Options) ctrl.Options {

	cacheWatchedLabelSelector := labels.SelectorFromSet(
		labels.Set{constant.ODLMWatchedLabel: ""},
	)
	cacheFreshLabelSelector := labels.SelectorFromSet(
		labels.Set{constant.BindInfoRefreshLabel: ""},
	)

	watchNamespace := util.GetWatchNamespace()
	watchNamespaces := strings.Split(watchNamespace, ",")

	// set DefaultNamespaces based on watchNamespaces
	// if watchNamespaces is empty, then cache resource in all namespaces
	var cacheDefaultNamespaces map[string]cache.Config
	if len(watchNamespaces) == 1 && watchNamespaces[0] == "" {
		// cache resource in all namespaces
		cacheDefaultNamespaces = map[string]cache.Config{corev1.NamespaceAll: {}}
	} else {
		// cache resource in watchNamespaces
		cacheNamespaces := make(map[string]cache.Config)
		for _, ns := range watchNamespaces {
			cacheNamespaces[ns] = cache.Config{}
		}
		cacheDefaultNamespaces = cacheNamespaces
	}

	// set byObject to watch the resources
	// the cache will watch the resources with the label selector
	cacheByObject := map[client.Object]cache.ByObject{
		&corev1.Secret{}:      {Label: cacheWatchedLabelSelector},
		&corev1.ConfigMap{}:   {Label: cacheWatchedLabelSelector},
		&appsv1.Deployment{}:  {Label: cacheFreshLabelSelector},
		&appsv1.DaemonSet{}:   {Label: cacheFreshLabelSelector},
		&appsv1.StatefulSet{}: {Label: cacheFreshLabelSelector},
	}

	// set SyncPeriod to the default cache sync period
	syncPeriod := constant.DefaultSyncPeriod

	// set cache options
	opts.Cache = cache.Options{
		ByObject:          cacheByObject,
		DefaultNamespaces: cacheDefaultNamespaces,
		SyncPeriod:        &syncPeriod,
		Scheme:            opts.Scheme,
	}

	return opts
}
