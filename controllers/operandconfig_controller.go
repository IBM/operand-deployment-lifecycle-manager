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

package controllers

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	fetch "github.com/IBM/operand-deployment-lifecycle-manager/controllers/common"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/util"
)

// OperandConfigReconciler reconciles a OperandConfig object
type OperandConfigReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
}

// +kubebuilder:rbac:groups=*,resources=*,verbs=*

// Reconcile reads that state of the cluster for a OperandConfig object and makes changes based on the state read
// and what is in the OperandConfig.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *OperandConfigReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	// Fetch the OperandConfig instance
	instance := &operatorv1alpha1.OperandConfig{}
	if err := r.Get(context.TODO(), req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	klog.V(1).Infof("Reconciling OperandConfig: %s", req.NamespacedName)

	// Set the init status for OperandConfig instance
	if !instance.InitConfigStatus() {
		klog.V(3).Infof("Initializing the status of OperandConfig: %s", req.NamespacedName)
		if err := r.updateOperandConfigStatus(instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.updateConfigOperatorsStatus(instance); err != nil {
		klog.Errorf("failed to update the status for OperandConfig %s/%s : %v", req.NamespacedName.Namespace, req.NamespacedName.Name, err)
		return ctrl.Result{}, err
	}

	// Check if all the services are deployed
	if instance.Status.Phase != operatorv1alpha1.ServiceInit &&
		instance.Status.Phase != operatorv1alpha1.ServiceRunning {
		klog.V(2).Info("Waiting for all the services being deployed ...")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	klog.V(1).Infof("Finished reconciling OperandConfig: %s", req.NamespacedName)
	return ctrl.Result{}, nil
}

func (r *OperandConfigReconciler) updateConfigOperatorsStatus(instance *operatorv1alpha1.OperandConfig) error {
	// Create an empty ServiceStatus map
	klog.V(3).Info("Initializing OperandConfig status")
	instance.Status.ServiceStatus = make(map[string]operatorv1alpha1.CrStatus)

	registryInstance, err := fetch.FetchOperandRegistry(r.Client, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace})
	if err != nil {
		return err
	}

	for _, op := range registryInstance.Spec.Operators {
		// Looking for the CSV
		namespace := fetch.GetOperatorNamespace(op.InstallMode, op.Namespace)
		sub, err := fetch.FetchSubscription(r.Client, op.Name, namespace, op.PackageName)

		if errors.IsNotFound(err) {
			klog.V(3).Infof("There is no Subscription %s or %s in the namespace %s", op.Name, op.PackageName, namespace)
			continue
		}

		if err != nil {
			klog.Errorf("failed to fetch Subscription %s or %s in the namespace %s", op.Name, op.PackageName, namespace)
			return err
		}

		if _, ok := sub.Labels[constant.OpreqLabel]; !ok {
			// Subscription existing and not managed by OperandRequest controller
			klog.V(2).Infof("Subscription %s in the namespace %s isn't created by ODLM", sub.Name, sub.Namespace)
		}

		csv, err := fetch.FetchClusterServiceVersion(r.Client, sub)

		if err != nil {
			klog.Errorf("failed to fetch ClusterServiceVersion for the Subscription %s in the namespace %s", sub.Name, namespace)
			return err
		}

		if csv == nil {
			klog.Warningf("ClusterServiceVersion for the Subscription %s in the namespace %s doesn't exist, retry...", sub.Name, namespace)
			continue
		}

		_, ok := instance.Status.ServiceStatus[op.Name]

		if !ok {
			instance.Status.ServiceStatus[op.Name] = operatorv1alpha1.CrStatus{}
		}

		if instance.Status.ServiceStatus[op.Name].CrStatus == nil {
			tmp := instance.Status.ServiceStatus[op.Name]
			tmp.CrStatus = make(map[string]operatorv1alpha1.ServicePhase)
			instance.Status.ServiceStatus[op.Name] = tmp
		}

		almExamples := csv.ObjectMeta.Annotations["alm-examples"]

		// Create a slice for crTemplates
		var crTemplates []interface{}

		// Convert CR template string to slice
		err = json.Unmarshal([]byte(almExamples), &crTemplates)
		if err != nil {
			klog.Errorf("failed to convert alm-examples in the Subscription %s to slice: %s", sub.Name, err)
			return err
		}

		merr := &util.MultiErr{}

		// Merge OperandConfig and ClusterServiceVersion alm-examples
		for _, crTemplate := range crTemplates {

			// Create an unstruct object for CR and request its value to CR template
			var unstruct unstructured.Unstructured
			unstruct.Object = crTemplate.(map[string]interface{})

			kind := unstruct.Object["kind"].(string)

			service := instance.GetService(op.Name)

			existinConfig := false
			for crName := range service.Spec {
				// Compare the name of OperandConfig and CRD name
				if strings.EqualFold(kind, crName) {
					existinConfig = true
				}
			}

			if !existinConfig {
				continue
			}

			name := unstruct.Object["metadata"].(map[string]interface{})["name"].(string)

			getError := r.Get(context.TODO(), types.NamespacedName{
				Name:      name,
				Namespace: op.Namespace,
			}, &unstruct)

			if getError != nil && !errors.IsNotFound(getError) {
				instance.Status.ServiceStatus[op.Name].CrStatus[kind] = operatorv1alpha1.ServiceFailed
			} else if errors.IsNotFound(getError) {
			} else {
				instance.Status.ServiceStatus[op.Name].CrStatus[kind] = operatorv1alpha1.ServiceRunning
			}
		}
		if len(merr.Errors) != 0 {
			return merr
		}
	}

	klog.V(2).Info("Updating OperandConfig status")

	instance.UpdateOperandPhase()

	if err := r.updateOperandConfigStatus(instance); err != nil {
		return err
	}

	return nil
}

func (r *OperandConfigReconciler) getRequestToConfigMapper() handler.ToRequestsFunc {
	return func(object handler.MapObject) []reconcile.Request {
		opreqInstance := &operatorv1alpha1.OperandRequest{}
		requests := []reconcile.Request{}
		// If the OperandRequest has been deleted, reconcile all the OperandConfig in the cluster
		if err := r.Get(context.TODO(), types.NamespacedName{Name: object.Meta.GetName(), Namespace: object.Meta.GetNamespace()}, opreqInstance); errors.IsNotFound(err) {
			configList := &operatorv1alpha1.OperandConfigList{}
			_ = r.List(context.TODO(), configList)
			for _, config := range configList.Items {
				namespaceName := types.NamespacedName{Name: config.Name, Namespace: config.Namespace}
				req := reconcile.Request{NamespacedName: namespaceName}
				requests = append(requests, req)
			}
			return requests
		}

		// If the OperandRequest exist, reconcile OperandConfigs specific in the OperandRequest instance.
		for _, request := range opreqInstance.Spec.Requests {
			registryKey := opreqInstance.GetRegistryKey(request)
			req := reconcile.Request{NamespacedName: registryKey}
			requests = append(requests, req)
		}
		return requests
	}
}

// SetupWithManager adds OperandConfig controller to the manager.
func (r *OperandConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.OperandConfig{}).
		Watches(&source.Kind{Type: &operatorv1alpha1.OperandRequest{}}, &handler.EnqueueRequestsFromMapFunc{
			ToRequests: r.getRequestToConfigMapper(),
		}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				// Evaluates to false if the object has been confirmed deleted.
				return !e.DeleteStateUnknown
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldObject := e.ObjectOld.(*operatorv1alpha1.OperandRequest)
				newObject := e.ObjectNew.(*operatorv1alpha1.OperandRequest)
				return !reflect.DeepEqual(oldObject.Status, newObject.Status)
			},
		})).Complete(r)
}

func (r *OperandConfigReconciler) updateOperandConfigStatus(newConfigInstance *operatorv1alpha1.OperandConfig) error {
	err := wait.PollImmediate(time.Millisecond*250, time.Second*5, func() (bool, error) {
		existingConfigInstance, err := fetch.FetchOperandConfig(r.Client, types.NamespacedName{Name: newConfigInstance.Name, Namespace: newConfigInstance.Namespace})
		if err != nil {
			klog.Error("failed to fetch the existing OperandConfig: ", err)
			return false, err
		}

		existingStatus := existingConfigInstance.Status.DeepCopy()
		newStatus := newConfigInstance.Status.DeepCopy()
		if reflect.DeepEqual(existingStatus, newStatus) {
			return true, nil
		}
		existingConfigInstance.Status = *newStatus
		if err := r.Status().Update(context.TODO(), existingConfigInstance); err != nil {
			return false, err
		}
		return true, nil
	})

	if err != nil {
		klog.Error("update OperandConfig status failed: ", err)
		return err
	}
	return nil
}
