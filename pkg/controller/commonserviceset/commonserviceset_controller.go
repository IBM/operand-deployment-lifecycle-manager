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

package commonserviceset

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	olmv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1"
	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/common-service-operator/pkg/apis/operator/v1alpha1"
	util "github.com/IBM/common-service-operator/pkg/util"
)

var log = logf.Log.WithName("controller_commonserviceset")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new CommonServiceSet Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	olmClientset, err := olmclient.NewForConfig(mgr.GetConfig())
	if err != nil {
		log.Error(err, "Initialize the OLM client failed.")
		return nil
	}
	return &ReconcileCommonServiceSet{
		client:    mgr.GetClient(),
		recorder:  mgr.GetEventRecorderFor("commonserviceset"),
		scheme:    mgr.GetScheme(),
		olmClient: olmClientset}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("commonserviceset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource CommonServiceSet
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.CommonServiceSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MetaOperator
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.MetaOperator{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource CommonServiceConfig
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.CommonServiceConfig{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner CommonServiceSet
	err = c.Watch(&source.Kind{Type: &olmv1alpha1.Subscription{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.CommonServiceSet{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &olmv1.OperatorGroup{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.CommonServiceSet{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileCommonServiceSet implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileCommonServiceSet{}

// ReconcileCommonServiceSet reconciles a CommonServiceSet object
type ReconcileCommonServiceSet struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client    client.Client
	recorder  record.EventRecorder
	scheme    *runtime.Scheme
	olmClient *olmclient.Clientset
}

type clusterObjects struct {
	namespace     *corev1.Namespace
	operatorGroup *olmv1.OperatorGroup
	subscription  *olmv1alpha1.Subscription
}

// Reconcile reads that state of the cluster for a CommonServiceSet object and makes changes based on the state read
// and what is in the CommonServiceSet.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileCommonServiceSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling CommonServiceSet")

	// Fetch the CommonServiceSet instance
	setInstance := &operatorv1alpha1.CommonServiceSet{}
	if err := r.client.Get(context.TODO(), request.NamespacedName, setInstance); err != nil {
		// Error reading the object - requeue the request.
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	// Add finalizer
	if setInstance.GetFinalizers() == nil {
		if err := r.addFinalizer(setInstance); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Fetch the MetaOperator instance
	mo := &operatorv1alpha1.MetaOperator{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: request.Namespace, Name: "common-service"}, mo); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch all subscription definition
	opts, err := r.fetchOperators(mo, setInstance)
	if opts == nil {
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	for _, o := range opts {
		// Check subscription if exist
		found, err := r.olmClient.OperatorsV1alpha1().Subscriptions(o.Namespace).Get(o.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				// Subscription does not exist and state is present, create a new one
				if o.State == Present {
					if err = r.createSubscription(setInstance, o); err != nil {
						return reconcile.Result{}, err
					}
				}
				continue
			}
			return reconcile.Result{}, err
		}

		// Subscription existing and managed by Set controller
		if _, ok := found.Labels["operator.ibm.com/css-control"]; ok {
			// Check subscription if present
			if o.State == Present {
				// Subscription is present and channel changed, update it.
				if found.Spec.Channel != o.Channel {
					found.Spec.Channel = o.Channel
					if err = r.updateSubscription(setInstance, found); err != nil {
						return reconcile.Result{}, err
					}
				}
			} else {
				// // Subscription is absent, delete it.
				if err := r.deleteSubscription(setInstance, found, mo); err != nil {
					return reconcile.Result{}, err
				}
			}
		} else {
			// Subscription existing and not managed by Set controller
			reqLogger.WithValues("Subscription.Namespace", found.Namespace, "Subscription.Name", found.Name).Info("Subscription has created by other user, ignore create it.")
		}
	}

	csc := &operatorv1alpha1.CommonServiceConfig{}
	if err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: request.Namespace, Name: "common-service"}, csc); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	// Fetch CommonServiceConfig instance
	serviceConfigs, err := r.fetchConfigs(request, setInstance)
	if serviceConfigs == nil {
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// Fetch Subscriptions and check the status of install plan
	err = r.waitForInstallPlan(mo)
	if err != nil {
		return reconcile.Result{}, err
	}

	merr := &multiErr{}

	// Reconcile the CommonServiceConfig
	for _, service := range serviceConfigs {
		if service.State == Present {
			reqLogger.Info(fmt.Sprintf("Reconciling custome resource %s", service.Name))
			// Looking for the CSV
			csv, err := r.getClusterServiceVersion(service.Name)

			// If can't get CSV, requeue the request
			if err != nil {
				merr.Add(err)
				continue
			}

			if csv == nil {
				continue
			}

			reqLogger.Info(fmt.Sprintf("Generating custome resource based on CSV %s", csv.ObjectMeta.Name))

			// Merge and Generate CR
			err = r.generateCr(service, csv, csc)
			if err != nil {
				merr.Add(err)
			}
		}
	}

	if len(merr.errors) != 0 {
		return reconcile.Result{}, merr
	}

	// Check unready number of subscription
	if err := r.updateMemberStatus(setInstance); err != nil {
		return reconcile.Result{}, err
	}
	if setInstance.Status.Members.Unready != nil {
		if err := r.updatePhaseStatus(setInstance, operatorv1alpha1.ClusterPhaseCreating); err != nil {
			return reconcile.Result{}, err
		}
		reqLogger.Info("Waiting for all the operators ready ......")
		return reconcile.Result{Requeue: true}, nil
	}
	if err := r.updatePhaseStatus(setInstance, operatorv1alpha1.ClusterPhaseRunning); err != nil {
		return reconcile.Result{}, err
	}

	// Remove finalizer
	if !setInstance.ObjectMeta.DeletionTimestamp.IsZero() {
		// Update finalizer to allow delete CR
		setInstance.SetFinalizers(nil)
		err := r.client.Update(context.TODO(), setInstance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileCommonServiceSet) fetchOperators(mo *operatorv1alpha1.MetaOperator, cr *operatorv1alpha1.CommonServiceSet) (map[string]operatorv1alpha1.Operator, error) {

	setMap, err := r.fetchSets(cr)
	if err != nil {
		return nil, err
	}

	optMap := make(map[string]operatorv1alpha1.Operator)
	for _, v := range mo.Spec.Operators {
		if _, ok := setMap[v.Name]; ok {
			if setMap[v.Name].Channel != "" && setMap[v.Name].Channel != v.Channel {
				v.Channel = setMap[v.Name].Channel
			}
			v.State = setMap[v.Name].State
		} else {
			v.State = Absent
		}
		optMap[v.Name] = v
	}
	return optMap, nil
}

func (r *ReconcileCommonServiceSet) fetchConfigs(req reconcile.Request, cr *operatorv1alpha1.CommonServiceSet) (map[string]operatorv1alpha1.ConfigService, error) {
	// Fetch the CommonServiceConfig instance
	csc := &operatorv1alpha1.CommonServiceConfig{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: req.Namespace, Name: "common-service"}, csc); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	setMap, err := r.fetchSets(cr)
	if err != nil {
		return nil, err
	}

	cscMap := make(map[string]operatorv1alpha1.ConfigService)
	for k, v := range csc.Spec.Services {
		if _, ok := setMap[v.Name]; ok {
			csc.Spec.Services[k].State = setMap[v.Name].State
		} else {
			csc.Spec.Services[k].State = Absent
		}
		cscMap[v.Name] = v
	}
	if err := r.client.Update(context.TODO(), csc); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return cscMap, nil
}

func (r *ReconcileCommonServiceSet) waitForInstallPlan(mo *operatorv1alpha1.MetaOperator) error {
	reqLogger := log.WithValues()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()
	return wait.PollImmediateUntil(time.Second*5, func() (bool, error) {
		foundSub, err := r.olmClient.OperatorsV1alpha1().Subscriptions("").List(metav1.ListOptions{
			LabelSelector: "operator.ibm.com/css-control",
		})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		for _, sub := range foundSub.Items {
			if sub.Status.Install == nil {
				reqLogger.Info("Waiting for Install Plan of " + sub.ObjectMeta.Name + " is ready")
				return false, nil
			}

			ip, err := r.olmClient.OperatorsV1alpha1().InstallPlans(sub.Namespace).Get(sub.Status.InstallPlanRef.Name, metav1.GetOptions{})

			if err != nil {
				err := r.updateOperatorStatus(mo, sub.ObjectMeta.Name, operatorv1alpha1.OperatorFailed)
				return false, err
			}

			if ip.Status.Phase != olmv1alpha1.InstallPlanPhaseComplete {
				reqLogger.Info("Waiting for Cluster Service Version of " + ip.Spec.ClusterServiceVersionNames[0] + " is ready")
				return false, nil
			}

			err = r.updateOperatorStatus(mo, sub.ObjectMeta.Name, operatorv1alpha1.OperatorRunning)
			if err != nil {
				return false, err
			}
		}
		return true, nil
	}, ctx.Done())
}

func (r *ReconcileCommonServiceSet) fetchSets(currentCr *operatorv1alpha1.CommonServiceSet) (map[string]operatorv1alpha1.SetService, error) {
	crs := &operatorv1alpha1.CommonServiceSetList{}
	if err := r.client.List(context.TODO(), crs); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	// If current CR is deleted, mark all the operators as absent state
	isCurrentCrToBeDeleted := currentCr.GetDeletionTimestamp() != nil
	sets := make(map[string]operatorv1alpha1.SetService)
	for _, cr := range crs.Items {
		for _, s := range cr.Spec.Services {
			if isCurrentCrToBeDeleted && cr.GetUID() == currentCr.GetUID() {
				s.State = Absent
			}
			sets = addSet(sets, s)
		}
	}
	return sets, nil
}

func addSet(sets map[string]operatorv1alpha1.SetService, set operatorv1alpha1.SetService) map[string]operatorv1alpha1.SetService {
	if _, ok := sets[set.Name]; ok {
		if set.State == Present {
			sets[set.Name] = set
		}
		return sets
	}
	sets[set.Name] = set
	return sets
}

func (r *ReconcileCommonServiceSet) createSubscription(cr *operatorv1alpha1.CommonServiceSet, opt operatorv1alpha1.Operator) error {
	logger := log.WithValues("Subscription.Namespace", opt.Namespace, "Subscription.Name", opt.Name)
	co := generateClusterObjects(opt)

	// Create required namespace
	ns := co.namespace
	if err := r.client.Create(context.TODO(), ns); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// Create required operatorgroup
	og := co.operatorGroup
	_, err := r.olmClient.OperatorsV1().OperatorGroups(og.Namespace).Create(og)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			if _, err = r.olmClient.OperatorsV1().OperatorGroups(og.Namespace).Update(og); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Create subscription
	logger.Info("Creating a new Subscription")
	sub := co.subscription
	_, err = r.olmClient.OperatorsV1alpha1().Subscriptions(sub.Namespace).Create(sub)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			if _, err = r.olmClient.OperatorsV1alpha1().Subscriptions(sub.Namespace).Update(sub); err != nil {
				if updateErr := r.updateConditionStatus(cr, sub.Name, InstallFailed); updateErr != nil {
					return updateErr
				}
				return err
			}
		} else {
			if updateErr := r.updateConditionStatus(cr, sub.Name, InstallFailed); updateErr != nil {
				return updateErr
			}
			return err
		}
	}

	if err = r.updateConditionStatus(cr, sub.Name, InstallSuccessed); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileCommonServiceSet) updateSubscription(cr *operatorv1alpha1.CommonServiceSet, sub *olmv1alpha1.Subscription) error {
	logger := log.WithValues("Subscription.Namespace", sub.Namespace, "Subscription.Name", sub.Name)

	logger.Info("Updating Subscription")
	if _, err := r.olmClient.OperatorsV1alpha1().Subscriptions(sub.Namespace).Update(sub); err != nil {
		if updateErr := r.updateConditionStatus(cr, sub.Name, UpdateFailed); updateErr != nil {
			return updateErr
		}
		return err
	}

	if err := r.updateConditionStatus(cr, sub.Name, UpdateSuccessed); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileCommonServiceSet) deleteSubscription(cr *operatorv1alpha1.CommonServiceSet, sub *olmv1alpha1.Subscription, mo *operatorv1alpha1.MetaOperator) error {
	logger := log.WithValues("Subscription.Namespace", sub.Namespace, "Subscription.Name", sub.Name)
	logger.Info("Deleting CSV related with Subscription")
	installedCsv := sub.Status.InstalledCSV
	if err := r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(sub.Namespace).Delete(installedCsv, &metav1.DeleteOptions{}); err != nil {
		if updateErr := r.updateConditionStatus(cr, sub.Name, DeleteFailed); updateErr != nil {
			return updateErr
		}
		return err
	}
	logger.Info("Deleting a Subscription")
	if err := r.olmClient.OperatorsV1alpha1().Subscriptions(sub.Namespace).Delete(sub.Name, &metav1.DeleteOptions{}); err != nil {
		if updateErr := r.updateConditionStatus(cr, sub.Name, DeleteFailed); updateErr != nil {
			return updateErr
		}
		return err
	}
	if err := r.updateConditionStatus(cr, sub.Name, DeleteSuccessed); err != nil {
		return err
	}
	if err := r.deleteOperatorStatus(mo, sub.Name); err != nil {
		return err
	}
	return nil
}

func generateClusterObjects(o operatorv1alpha1.Operator) *clusterObjects {
	co := &clusterObjects{}
	labels := map[string]string{
		"operator.ibm.com/css-control": "true",
	}
	// Namespace Object
	co.namespace = &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   o.Namespace,
			Labels: labels,
		},
	}

	// Operator Group Object
	og := &olmv1.OperatorGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "common-service-operatorgroup",
			Namespace: o.Namespace,
			Labels:    labels,
		},
		Spec: olmv1.OperatorGroupSpec{
			TargetNamespaces: o.TargetNamespaces,
		},
	}
	og.SetGroupVersionKind(schema.GroupVersionKind{Group: olmv1.SchemeGroupVersion.Group, Kind: "OperatorGroup", Version: olmv1.SchemeGroupVersion.Version})
	co.operatorGroup = og

	// Subscription Object
	installPlanApproval := olmv1alpha1.ApprovalAutomatic
	if o.InstallPlanApproval == "Manual" {
		installPlanApproval = olmv1alpha1.ApprovalManual
	}
	sub := &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Name,
			Namespace: o.Namespace,
			Labels:    labels,
		},
		Spec: &olmv1alpha1.SubscriptionSpec{
			Channel:                o.Channel,
			Package:                o.PackageName,
			CatalogSource:          o.SourceName,
			CatalogSourceNamespace: o.SourceNamespace,
			InstallPlanApproval:    installPlanApproval,
		},
	}
	sub.SetGroupVersionKind(schema.GroupVersionKind{Group: olmv1alpha1.SchemeGroupVersion.Group, Kind: "Subscription", Version: olmv1alpha1.SchemeGroupVersion.Version})
	co.subscription = sub
	return co
}

func (r *ReconcileCommonServiceSet) addFinalizer(cr *operatorv1alpha1.CommonServiceSet) error {
	if len(cr.GetFinalizers()) < 1 && cr.GetDeletionTimestamp() == nil {
		cr.SetFinalizers([]string{"finalizer.set.ibm.com"})
		// Update CR
		err := r.client.Update(context.TODO(), cr)
		if err != nil {
			return err
		}
	}
	return nil
}

// getCSV retrieves the Cluster Service Version
func (r *ReconcileCommonServiceSet) getClusterServiceVersion(subName string) (*olmv1alpha1.ClusterServiceVersion, error) {
	logger := log.WithValues("Subscription Name", subName)
	logger.Info(fmt.Sprintf("Looking for the Cluster Service Version"))
	subs, listSubErr := r.olmClient.OperatorsV1alpha1().Subscriptions("").List(metav1.ListOptions{
		LabelSelector: "operator.ibm.com/css-control",
	})
	if listSubErr != nil {
		logger.Error(listSubErr, "Fail to list subscriptions")
		return nil, listSubErr
	}
	var csvName, csvNamespace string
	for _, s := range subs.Items {
		if s.Name == subName {
			csvName = s.Status.CurrentCSV
			csvNamespace = s.Namespace
			csv, getCSVErr := r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(csvNamespace).Get(csvName, metav1.GetOptions{})
			if getCSVErr != nil {
				if errors.IsNotFound(getCSVErr) {
					continue
				}
				logger.Error(getCSVErr, "Fail to get Cluster Service Version")
				return nil, getCSVErr
			}
			logger.Info(fmt.Sprintf("Get Cluster Service Version %s in namespace %s", csvName, csvNamespace))
			return csv, nil
		}
	}
	logger.Info(fmt.Sprintf("Can't find Cluster Service Version"))
	return nil, nil
}

// generateCr merge and create custome resource base on CommonServiceConfig and CSV alm-examples
func (r *ReconcileCommonServiceSet) generateCr(service operatorv1alpha1.ConfigService, csv *olmv1alpha1.ClusterServiceVersion, csc *operatorv1alpha1.CommonServiceConfig) error {
	almExamples := csv.ObjectMeta.Annotations["alm-examples"]
	namespace := csv.ObjectMeta.Namespace
	logger := log.WithValues("Subscription", service.Name)

	// Create a slice for crTemplates
	var crTemplates []interface{}

	// Convert CR template string to slice
	crTemplatesErr := json.Unmarshal([]byte(almExamples), &crTemplates)
	if crTemplatesErr != nil {
		logger.Error(crTemplatesErr, "Fail to convert alm-examples to slice")
		return crTemplatesErr
	}

	merr := &multiErr{}

	// Merge CommonServiceConfig and Cluster Service Version alm-examples
	for _, crTemplate := range crTemplates {

		// Create an unstruct object for CR and set its value to CR template
		var unstruct unstructured.Unstructured
		unstruct.Object = crTemplate.(map[string]interface{})

		// Get the kind of CR
		name := unstruct.Object["kind"]

		for crdName, crConfig := range service.Spec {

			// Compare the name of CommonServiceConfig and CRD name
			if strings.EqualFold(name.(string), crdName) {
				logger.Info(fmt.Sprintf("Found CommonServiceConfig for %s", name))
				//Convert CR template spec to string
				specJSONString, _ := json.Marshal(unstruct.Object["spec"])

				// Merge CR template spec and CommonServiceConfig spec
				mergedCR := util.MergeCR(specJSONString, crConfig.Raw)

				unstruct.Object["spec"] = mergedCR
				unstruct.Object["metadata"].(map[string]interface{})["namespace"] = namespace

				setControllerErr := controllerutil.SetControllerReference(csv, &unstruct, r.scheme)

				if setControllerErr != nil {
					stateUpdateErr := r.updateServiceStatus(csc, service.Name, crdName, operatorv1alpha1.ServiceFailed)
					if stateUpdateErr != nil {
						merr.Add(stateUpdateErr)
					}
					logger.Error(setControllerErr, "Fail to set owner for "+crdName+".")
					merr.Add(setControllerErr)
					continue
				}
				// Creat or Update the CR
				crCreateErr := r.client.Create(context.TODO(), &unstruct)
				if crCreateErr != nil && !errors.IsAlreadyExists(crCreateErr) {
					stateUpdateErr := r.updateServiceStatus(csc, service.Name, crdName, operatorv1alpha1.ServiceFailed)
					if stateUpdateErr != nil {
						merr.Add(stateUpdateErr)
					}
					logger.Error(crCreateErr, "Fail to Create the CR "+crdName)
					merr.Add(crCreateErr)

				} else if errors.IsAlreadyExists(crCreateErr) {
					existingCR := &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": unstruct.Object["apiVersion"].(string),
							"kind":       unstruct.Object["kind"].(string),
						},
					}

					crGetErr := r.client.Get(context.TODO(), types.NamespacedName{
						Name:      unstruct.Object["metadata"].(map[string]interface{})["name"].(string),
						Namespace: namespace,
					}, existingCR)

					if crGetErr != nil {
						stateUpdateErr := r.updateServiceStatus(csc, service.Name, crdName, operatorv1alpha1.ServiceFailed)
						if stateUpdateErr != nil {
							merr.Add(stateUpdateErr)
						}
						logger.Error(crGetErr, "Fail to Get the CR "+crdName)
						merr.Add(crGetErr)
						continue
					}
					unstruct.Object["metadata"] = existingCR.Object["metadata"]
					if crUpdateErr := r.client.Update(context.TODO(), &unstruct); crUpdateErr != nil {
						stateUpdateErr := r.updateServiceStatus(csc, service.Name, crdName, operatorv1alpha1.ServiceFailed)
						if stateUpdateErr != nil {
							merr.Add(stateUpdateErr)
						}
						logger.Error(crUpdateErr, "Fail to Update the CR "+crdName)
						merr.Add(crUpdateErr)
						continue
					}
					logger.Info("Updated the CR " + crdName)
					stateUpdateErr := r.updateServiceStatus(csc, service.Name, crdName, operatorv1alpha1.ServiceRunning)
					if stateUpdateErr != nil {
						merr.Add(stateUpdateErr)
					}

				} else {
					logger.Info("Created the CR " + crdName)
					stateUpdateErr := r.updateServiceStatus(csc, service.Name, crdName, operatorv1alpha1.ServiceRunning)
					if stateUpdateErr != nil {
						merr.Add(stateUpdateErr)
					}
				}
			}
		}
	}

	if len(merr.errors) != 0 {
		return merr
	}

	return nil
}

type multiErr struct {
	errors []string
}

func (mer *multiErr) Error() string {
	return "Commn services installation errors : " + strings.Join(mer.errors, ":")
}

func (mer *multiErr) Add(err error) {
	if mer.errors == nil {
		mer.errors = []string{}
	}
	mer.errors = append(mer.errors, err.Error())
}
