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

package main

import (
	"flag"
	"os"
	"strings"

	ocproute "github.com/openshift/api/route/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	cache "github.com/IBM/controller-filtered-cache/filteredcache"
	nssv1 "github.com/IBM/ibm-namespace-scope-operator/api/v1"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/k8sutil"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/namespacescope"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operandbindinfo"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operandconfig"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operandregistry"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operandrequest"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operandrequestnoolm"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operator"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operatorchecker"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operatorconfig"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/util"
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
	utilruntime.Must(operatorsv1.AddToScheme(scheme))
	utilruntime.Must(ocproute.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()
	var metricsAddr string
	var probeAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	var stepSize = flag.Int("batch-chunk-size", 3, "batch-chunk-size is used to control at most how many subscriptions will be created concurrently")

	flag.Parse()

	gvkLabelMap := map[schema.GroupVersionKind]cache.Selector{
		corev1.SchemeGroupVersion.WithKind("Secret"): {
			LabelSelector: constant.ODLMWatchedLabel,
		},
		corev1.SchemeGroupVersion.WithKind("ConfigMap"): {
			LabelSelector: constant.ODLMWatchedLabel,
		},
		appsv1.SchemeGroupVersion.WithKind("Deployment"): {
			LabelSelector: constant.BindInfoRefreshLabel,
		},
		appsv1.SchemeGroupVersion.WithKind("StatefulSet"): {
			LabelSelector: constant.BindInfoRefreshLabel,
		},
		appsv1.SchemeGroupVersion.WithKind("DaemonSet"): {
			LabelSelector: constant.BindInfoRefreshLabel,
		},
	}

	options := ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		HealthProbeBindAddress: probeAddr,
		Port:                   9443,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "ab89bbb1.ibm.com",
	}

	watchNamespace := util.GetWatchNamespace()
	// isolatedModeEnable := util.GetIsolatedMode()
	isolatedModeEnable := true
	operatorCheckerDisable := util.GetoperatorCheckerMode()
	options.NewCache = k8sutil.NewODLMCache(isolatedModeEnable, strings.Split(watchNamespace, ","), gvkLabelMap)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		klog.Errorf("unable to start manager: %v", err)
		os.Exit(1)
	}
	noolm := util.GetNoOLM()
	if noolm == "true" {
		if err = (&operandrequestnoolm.Reconciler{
			ODLMOperator: deploy.NewODLMOperator(mgr, "OperandRequest"),
			StepSize:     *stepSize,
		}).SetupWithManager(mgr); err != nil {
			klog.Errorf("unable to create controller OperandRequestNoOLM: %v", err)
			os.Exit(1)
		}
	} else {
		if err = (&operandrequest.Reconciler{
			ODLMOperator: deploy.NewODLMOperator(mgr, "OperandRequest"),
			StepSize:     *stepSize,
		}).SetupWithManager(mgr); err != nil {
			klog.Errorf("unable to create controller OperandRequest: %v", err)
			os.Exit(1)
		}
	}
	if err = (&operandconfig.Reconciler{
		ODLMOperator: deploy.NewODLMOperator(mgr, "OperandConfig"),
	}).SetupWithManager(mgr); err != nil {
		klog.Errorf("unable to create controller OperandConfig: %v", err)
		os.Exit(1)
	}
	if err = (&operandbindinfo.Reconciler{
		ODLMOperator: deploy.NewODLMOperator(mgr, "OperandBindInfo"),
	}).SetupWithManager(mgr); err != nil {
		klog.Errorf("unable to create controller OperandBindInfo: %v", err)
		os.Exit(1)
	}
	if err = (&operandregistry.Reconciler{
		ODLMOperator: deploy.NewODLMOperator(mgr, "OperandRegistry"),
	}).SetupWithManager(mgr); err != nil {
		klog.Errorf("unable to create controller OperandRegistry: %v", err)
		os.Exit(1)
	}
	// Single instance case, disable it on SaaS or on-prem multi instances case
	if !isolatedModeEnable {
		if err = (&namespacescope.Reconciler{
			ODLMOperator: deploy.NewODLMOperator(mgr, "NamespaceScope"),
		}).SetupWithManager(mgr); err != nil {
			klog.Errorf("unable to create controller NamespaceScope: %v", err)
			os.Exit(1)
		}
	}
	if false {
		if !operatorCheckerDisable {
			if err = (&operatorchecker.Reconciler{
				ODLMOperator: deploy.NewODLMOperator(mgr, "OperatorChecker"),
			}).SetupWithManager(mgr); err != nil {
				klog.Errorf("unable to create controller OperatorChecker: %v", err)
				os.Exit(1)
			}
		}
	}
	if err = (&operatorconfig.Reconciler{
		ODLMOperator: deploy.NewODLMOperator(mgr, "OperatorConfig"),
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "OperatorConfig")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		klog.Errorf("unable to set up health check: %v", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		klog.Errorf("unable to set up ready check: %v", err)
		os.Exit(1)
	}

	klog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.Errorf("problem running manager: %v", err)
		os.Exit(1)
	}
}
