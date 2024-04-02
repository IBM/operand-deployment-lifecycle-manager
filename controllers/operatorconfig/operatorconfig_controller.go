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

package operatorconfig

import (
	"context"

	"github.com/barkimedes/go-deepcopy"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/controllers/operator"
)

// OperatorConfigReconciler reconciles a OperatorConfig object
type Reconciler struct {
	*deploy.ODLMOperator
}

//+kubebuilder:rbac:groups=operator.ibm.com,namespace="placeholder",resources=operatorconfigs;operatorconfigs/status;operatorconfigs/finalizers,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OperatorConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	instance := &operatorv1alpha1.OperandRequest{}
	if err := r.Client.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	klog.Infof("Reconciling OperatorConfig for request: %s, %s", instance.Namespace, instance.Name)

	for _, v := range instance.Spec.Requests {
		reqBlock := v
		registry, err := r.GetOperandRegistry(ctx, instance.GetRegistryKey(reqBlock))
		if err != nil {
			return ctrl.Result{}, err
		}
		for _, u := range reqBlock.Operands {
			operand := u
			operator := registry.GetOperator(operand.Name)
			if operator.OperatorConfig == "" {
				break
			}

			var sub *olmv1alpha1.Subscription
			sub, err = r.GetSubscription(ctx, operator.Name, operator.Namespace, registry.Namespace, operator.PackageName)
			if err != nil {
				return ctrl.Result{}, err
			}

			var csv *olmv1alpha1.ClusterServiceVersion
			csv, err = r.GetClusterServiceVersion(ctx, sub)
			if err != nil {
				return ctrl.Result{}, err
			}

			klog.Infof("Fetching OperatorConfig: %s", operator.OperatorConfig)
			config := &operatorv1alpha1.OperatorConfig{}
			if err := r.Client.Get(ctx, types.NamespacedName{
				Name:      operator.OperatorConfig,
				Namespace: registry.Namespace,
			}, config); err != nil {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
			serviceConfig := config.GetConfigForOperator(operator.Name)
			if serviceConfig == nil {
				klog.Infof("OperatorConfig: %s, does not have configuration for operator: %s", operator.OperatorConfig, operator.Name)
				return ctrl.Result{}, nil
			}

			copyToCast, err := deepcopy.Anything(csv)
			if err != nil {
				return ctrl.Result{}, err
			}
			csvToUpdate := copyToCast.(*olmv1alpha1.ClusterServiceVersion)
			klog.Infof("Applying OperatorConfig: %s to Operator: %s via CSV: %s, %s", operator.OperatorConfig, operator.Name, csv.Name, csv.Namespace)
			return r.configCsv(ctx, csvToUpdate, serviceConfig)
		}
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) configCsv(ctx context.Context, csv *olmv1alpha1.ClusterServiceVersion, config *operatorv1alpha1.ServiceOperatorConfig) (ctrl.Result, error) {
	if config.Replicas != nil {
		csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[0].Spec.Replicas = config.Replicas
	}
	if config.Affinity != nil {
		csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[0].Spec.Template.Spec.Affinity = config.Affinity
	}
	if config.TopologySpreadConstraints != nil {
		csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[0].Spec.Template.Spec.TopologySpreadConstraints = config.TopologySpreadConstraints
	}
	if err := r.Client.Update(ctx, csv); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) requestsFromMapFunc(ctx context.Context) handler.MapFunc {
	return func(object client.Object) []reconcile.Request {
		requests := []reconcile.Request{}

		operandRequests, _ := r.ListOperandRequests(ctx, nil)
		for _, req := range operandRequests.Items {
			r := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: req.Namespace,
					Name:      req.Name,
				},
			}
			requests = append(requests, r)
		}
		return requests
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.OperandRequest{}).
		Watches(&source.Kind{Type: &operatorv1alpha1.OperatorConfig{}}, handler.EnqueueRequestsFromMapFunc(r.requestsFromMapFunc(ctx)), builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				// Evaluates to false if the object has been confirmed deleted.
				return !e.DeleteStateUnknown
			},
		})).
		Complete(r)
}
