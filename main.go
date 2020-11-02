//
// Copyright 2020 IBM Corporation
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

package main

import (
	"flag"
	"os"
	"strings"

	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"

	cache "github.com/IBM/controller-filtered-cache/filteredcache"
	nssv1 "github.com/IBM/ibm-namespace-scope-operator/api/v1"
	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/k8sutil"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/util"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(olmv1.AddToScheme(scheme))
	utilruntime.Must(olmv1alpha1.AddToScheme(scheme))
	utilruntime.Must(nssv1.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	gvkLabelMap := map[schema.GroupVersionKind]cache.Selector{
		corev1.SchemeGroupVersion.WithKind("Secret"): {
			LabelSelector: constant.OpbiTypeLabel,
		},
		corev1.SchemeGroupVersion.WithKind("ConfigMap"): {
			LabelSelector: constant.OpbiTypeLabel,
		},
	}

	options := ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "ab89bbb1.ibm.com",
	}

	scope := util.GetInstallScope()
	watchNamespace := util.GetWatchNamespace()
	if scope == "namespaced" {
		options.NewCache = k8sutil.NewODLMCache(strings.Split(watchNamespace, ","))
	} else {
		options.NewCache = cache.NewFilteredCacheBuilder(gvkLabelMap)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		klog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.OperandRequestReconciler{
		Client:   mgr.GetClient(),
		Config:   mgr.GetConfig(),
		Recorder: mgr.GetEventRecorderFor("OperandRequest"),
		Scheme:   mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "OperandRequest")
		os.Exit(1)
	}
	if err = (&controllers.OperandConfigReconciler{
		Client:   mgr.GetClient(),
		Recorder: mgr.GetEventRecorderFor("OperandConfig"),
		Scheme:   mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "OperandConfig")
		os.Exit(1)
	}
	if err = (&controllers.OperandBindInfoReconciler{
		Client:   mgr.GetClient(),
		Recorder: mgr.GetEventRecorderFor("OperandBindInfo"),
		Scheme:   mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "OperandBindInfo")
		os.Exit(1)
	}
	if err = (&controllers.OperandRegistryReconciler{
		Client:   mgr.GetClient(),
		Recorder: mgr.GetEventRecorderFor("OperandRegistry"),
		Scheme:   mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "OperandRegistry")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	klog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
