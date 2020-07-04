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

package operandconfig

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	constant "github.com/IBM/operand-deployment-lifecycle-manager/pkg/constant"
	fetch "github.com/IBM/operand-deployment-lifecycle-manager/pkg/controller/common"
	util "github.com/IBM/operand-deployment-lifecycle-manager/pkg/util"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new OperandConfig Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	olmClientset, err := olmclient.NewForConfig(mgr.GetConfig())
	if err != nil {
		klog.Error("Initialize the OLM client failed: ", err)
		return nil
	}
	return &ReconcileOperandConfig{
		client:    mgr.GetClient(),
		scheme:    mgr.GetScheme(),
		olmClient: olmClientset,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("operandconfig-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for OperandRequest spec changes and requeue the OperandConfig
	if err = c.Watch(&source.Kind{Type: &operatorv1alpha1.OperandRequest{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: getRequestToConfigMapper(mgr),
	}, predicate.Funcs{
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
	}); err != nil {
		return err
	}

	// Watch for changes to primary resource OperandConfig
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.OperandConfig{}}, &handler.EnqueueRequestForObject{}, predicate.GenerationChangedPredicate{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileOperandConfig implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileOperandConfig{}

// ReconcileOperandConfig reconciles a OperandConfig object
type ReconcileOperandConfig struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client    client.Client
	scheme    *runtime.Scheme
	olmClient olmclient.Interface
}

// Reconcile reads that state of the cluster for a OperandConfig object and makes changes based on the state read
// and what is in the OperandConfig.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileOperandConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	// Fetch the OperandConfig instance
	instance := &operatorv1alpha1.OperandConfig{}
	if err := r.client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	klog.V(1).Infof("Reconciling OperandConfig %s", request.NamespacedName)

	// Set the init status for OperandConfig instance
	if !instance.InitConfigStatus() {
		klog.V(3).Infof("Initializing the status of OperandConfig %s in the namespace %s", request.Name, request.Namespace)
		if err := r.client.Status().Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	if err := r.updateConfigOperatorsStatus(instance); err != nil {
		return reconcile.Result{}, err
	}

	// Set Finalizer for the OperandConfig
	added := instance.EnsureFinalizer()
	if added {
		if err := r.client.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	if instance.DeletionTimestamp != nil {
		requestList, err := fetch.FetchAllOperandRequests(r.client, map[string]string{instance.Namespace + "." + instance.Name + "/config": "true"})
		if err != nil {
			return reconcile.Result{}, err
		}
		if len(requestList.Items) == 0 {
			removed := instance.RemoveFinalizer()
			if removed {
				if err := r.client.Update(context.TODO(), instance); err != nil {
					return reconcile.Result{}, err
				}
			}
		} else {
			return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
		}
	}

	// Fetch the OperandConfig instance
	instance = &operatorv1alpha1.OperandConfig{}
	if err := r.client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	// Check if all the services are deployed
	if instance.Status.Phase != operatorv1alpha1.ServiceRunning {
		klog.V(2).Info("Waiting for all the services being deployed ...")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileOperandConfig) updateConfigOperatorsStatus(instance *operatorv1alpha1.OperandConfig) error {
	// Create an empty ServiceStatus map
	klog.V(3).Info("Initializing OperandConfig status")
	instance.Status.ServiceStatus = make(map[string]operatorv1alpha1.CrStatus)

	registryInstance, err := fetch.FetchOperandRegistry(r.client, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace})
	if err != nil {
		return err
	}

	for _, op := range registryInstance.Spec.Operators {
		// Looking for the CSV
		namespace := fetch.GetOperatorNamespace(op.InstallMode, op.Namespace)
		sub, err := fetch.FetchSubscription(r.olmClient, op.Name, namespace, op.PackageName)

		if errors.IsNotFound(err) {
			klog.V(3).Infof("There is no Subscription %s or %s in the namespace %s", op.Name, op.PackageName, namespace)
			continue
		}

		if err != nil {
			klog.Errorf("Failed to fetch Subscription %s or %s in the namespace %s", op.Name, op.PackageName, namespace)
			return err
		}

		if _, ok := sub.Labels[constant.OpreqLabel]; !ok {
			// Subscription existing and not managed by OperandRequest controller
			klog.V(2).Infof("Subscription %s in the namespace %s isn't created by ODLM", sub.Name, sub.Namespace)
		}

		csv, err := fetch.FetchClusterServiceVersion(r.olmClient, sub)

		if err != nil {
			klog.Errorf("Failed to fetch ClusterServiceVersion for the Subscription %s in the namespace %s", sub.Name, namespace)
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
			klog.Errorf("Failed to convert alm-examples in the Subscription %s to slice: %s", sub.Name, err)
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

			getError := r.client.Get(context.TODO(), types.NamespacedName{
				Name:      name,
				Namespace: op.Namespace,
			}, &unstruct)

			if getError != nil && !errors.IsNotFound(getError) {
				instance.Status.ServiceStatus[op.Name].CrStatus[kind] = operatorv1alpha1.ServiceFailed
			} else if errors.IsNotFound(getError) {
				instance.Status.ServiceStatus[op.Name].CrStatus[kind] = operatorv1alpha1.ServiceNotReady
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

	if err := r.client.Status().Update(context.TODO(), instance); err != nil {
		return err
	}

	return nil
}

func getRequestToConfigMapper(mgr manager.Manager) handler.ToRequestsFunc {
	return func(object handler.MapObject) []reconcile.Request {
		mgrClient := mgr.GetClient()
		opreqInstance := &operatorv1alpha1.OperandRequest{}

		requests := []reconcile.Request{}

		// If the operandrequest has been deleted, reconcile all the OperandConfig in the cluster
		if err := mgrClient.Get(context.TODO(), types.NamespacedName{Name: object.Meta.GetName(), Namespace: object.Meta.GetNamespace()}, opreqInstance); errors.IsNotFound(err) {
			configList := &operatorv1alpha1.OperandConfigList{}
			_ = mgrClient.List(context.TODO(), configList)
			for _, config := range configList.Items {
				namespaceName := types.NamespacedName{Name: config.Name, Namespace: config.Namespace}
				req := reconcile.Request{NamespacedName: namespaceName}
				requests = append(requests, req)
			}
			return requests
		}

		// If the operandrequest exist, reconcile OperandConfigs specific in the operandrequest instance.
		for _, request := range opreqInstance.Spec.Requests {
			registryKey := opreqInstance.GetRegistryKey(request)
			req := reconcile.Request{NamespacedName: registryKey}
			requests = append(requests, req)
		}
		return requests
	}
}
