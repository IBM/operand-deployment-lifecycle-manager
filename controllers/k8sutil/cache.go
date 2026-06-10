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

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/util"
)

func NewODLMCache(isolatedModeEnable bool, opts ctrl.Options, config *rest.Config) ctrl.Options {

	cacheWatchedLabelSelector := labels.SelectorFromSet(
		labels.Set{constant.ODLMWatchedLabel: "true"},
	)
	cacheFreshLabelSelector := labels.SelectorFromSet(
		labels.Set{constant.BindInfoRefreshLabel: "enabled"},
	)

	watchNamespace := util.GetWatchNamespace()
	watchNamespaces := strings.Split(watchNamespace, ",")

	// set DefaultNamespaces based on watchNamespaces
	// if watchNamespaces is empty, then cache resource in all namespaces
	var cacheDefaultNamespaces map[string]cache.Config
	if len(watchNamespaces) == 1 && watchNamespaces[0] == "" {
		klog.Infof("All namespaces will be watched")
		// cache resource in all namespaces
		cacheDefaultNamespaces = nil
	} else {
		klog.Infof("Watching namespaces: %v", watchNamespaces)
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

	// Only cache OLM resources if OLM API exists in the cluster
	if util.IsOLMInstalled(config) {
		cacheByObject[&olmv1alpha1.ClusterServiceVersion{}] = cache.ByObject{} // Cache is needed because the action for deleting CSVs
		cacheByObject[&olmv1alpha1.Subscription{}] = cache.ByObject{}          // Cache all subscriptions for any labeled and unlabeled subscriptions
	}

	// set cache options
	opts.Cache = cache.Options{
		ByObject:          cacheByObject,
		DefaultNamespaces: cacheDefaultNamespaces,
		Scheme:            opts.Scheme,
	}

	return opts
}
