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

package v1alpha1

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/util"
)

func (r *OperandRequest) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-operator-ibm-com-v1alpha1-operandrequest,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.ibm.com,resources=operandrequests,verbs=create;update,versions=v1alpha1,name=moperandrequest.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &OperandRequest{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OperandRequest) Default() {
	for i, req := range r.Spec.Requests {
		regNs := req.RegistryNamespace
		if regNs == "" {
			regNs = r.Namespace
		}
		watchNamespace := util.GetWatchNamespace()
		isDefaulting := false
		// watchNamespace is empty in All namespace mode
		if len(watchNamespace) == 0 {
			cfg, err := config.GetConfig()
			if err != nil {
				klog.Errorf("Failed to get config: %v", err)
			} else {
				dynamic := dynamic.NewForConfigOrDie(cfg)

				resourceID := schema.GroupVersionResource{
					Group:    "",
					Version:  "v1",
					Resource: "namespaces",
				}
				ctx := context.Background()
				if _, err := dynamic.Resource(resourceID).Get(ctx, regNs, metav1.GetOptions{}); err != nil {
					if errors.IsNotFound(err) {
						klog.Infof("Not found registrySamespace %v for OperandRequest %v/%v", regNs, r.Namespace, r.Name)
						isDefaulting = true
					} else {
						klog.Errorf("Failed to get namespace %v: %v", regNs, err)
					}
				}
			}

		} else if len(watchNamespace) != 0 && !util.Contains(strings.Split(watchNamespace, ","), regNs) {
			isDefaulting = true
		}
		if isDefaulting {
			operatorNamespace := util.GetOperatorNamespace()
			r.Spec.Requests[i].RegistryNamespace = operatorNamespace
			klog.Infof("Setting %vth RegistryNamespace for OperandRequest %v/%v: %v", i, r.Namespace, r.Name, operatorNamespace)
		}
	}
	// TODO(user): fill in your defaulting logic.
}
