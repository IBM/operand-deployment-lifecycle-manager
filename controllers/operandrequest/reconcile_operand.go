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

package operandrequest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/pkg/errors"
	authorizationv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	constant "github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	util "github.com/IBM/operand-deployment-lifecycle-manager/controllers/util"
)

func (r *Reconciler) reconcileOperand(ctx context.Context, requestInstance *operatorv1alpha1.OperandRequest) *util.MultiErr {
	klog.V(1).Infof("Reconciling Operands for OperandRequest: %s/%s", requestInstance.GetNamespace(), requestInstance.GetName())
	// Update request status
	defer func() {
		requestInstance.UpdateClusterPhase()
	}()

	merr := &util.MultiErr{}
	if err := r.checkCustomResource(ctx, requestInstance); err != nil {
		merr.Add(err)
		return merr
	}
	for _, req := range requestInstance.Spec.Requests {
		registryKey := requestInstance.GetRegistryKey(req)
		registryInstance, err := r.GetOperandRegistry(ctx, registryKey)
		if err != nil {
			merr.Add(errors.Wrapf(err, "failed to get the OperandRegistry %s", registryKey.String()))
			continue
		}

		for i, operand := range req.Operands {

			opdRegistry := registryInstance.GetOperator(operand.Name)
			if opdRegistry == nil {
				klog.Warningf("Cannot find %s in the OperandRegistry instance %s in the namespace %s ", operand.Name, req.Registry, req.RegistryNamespace)
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorNotFound, operatorv1alpha1.ServiceNotFound, &r.Mutex)
				continue
			}

			operatorName := opdRegistry.Name

			klog.V(3).Info("Looking for csv for the operator: ", operatorName)

			// Looking for the CSV
			namespace := r.GetOperatorNamespace(opdRegistry.InstallMode, opdRegistry.Namespace)

			sub, err := r.GetSubscription(ctx, operatorName, namespace, registryInstance.Namespace, opdRegistry.PackageName)

			if sub == nil && err == nil {
				klog.Warningf("There is no Subscription %s or %s in the namespace %s and %s", operatorName, opdRegistry.PackageName, namespace, registryInstance.Namespace)
				continue

			} else if err != nil {
				merr.Add(errors.Wrapf(err, "failed to get the Subscription %s in the namespace %s and %s", operatorName, namespace, registryInstance.Namespace))
				return merr
			}

			if _, ok := sub.Labels[constant.OpreqLabel]; !ok {
				// Subscription existing and not managed by OperandRequest controller
				klog.Warningf("Subscription %s in the namespace %s isn't created by ODLM", sub.Name, sub.Namespace)
			}

			// It the installplan is not created yet, ODLM will try later
			if sub.Status.Install == nil || sub.Status.InstallPlanRef.Name == "" {
				klog.Warningf("The Installplan for Subscription %s is not ready. Will check it again", sub.Name)
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorInstalling, "", &r.Mutex)
				continue
			}

			// If the installplan is deleted after is completed, ODLM won't block the CR update.
			ipName := sub.Status.InstallPlanRef.Name
			ipNamespace := sub.Namespace
			ip := &olmv1alpha1.InstallPlan{}
			ipKey := types.NamespacedName{
				Name:      ipName,
				Namespace: ipNamespace,
			}
			if err := r.Client.Get(ctx, ipKey, ip); err != nil {
				if !apierrors.IsNotFound(err) {
					merr.Add(errors.Wrapf(err, "failed to get Installplan"))
				}
			} else if ip.Status.Phase == olmv1alpha1.InstallPlanPhaseFailed {
				klog.Errorf("installplan %s/%s is failed", ipNamespace, ipName)
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorFailed, "", &r.Mutex)
				continue
			}

			csv, err := r.GetClusterServiceVersion(ctx, sub)

			// If can't get CSV, requeue the request
			if err != nil {
				merr.Add(err)
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorFailed, "", &r.Mutex)
				continue
			}

			if csv == nil {
				klog.Warningf("ClusterServiceVersion for the Subscription %s in the namespace %s is not ready yet, retry", operatorName, namespace)
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorInstalling, "", &r.Mutex)
				continue
			}

			// find the OperandRequest which has the same operator's channel version as existing subscription.
			// ODLM will only reconcile Operand based on OperandConfig for this OperandRequest
			var requestList []string
			reg, _ := regexp.Compile(`^(.*)\.(.*)\.(.*)\/request`)
			for anno, version := range sub.Annotations {
				if reg.MatchString(anno) && version == sub.Spec.Channel {
					requestList = append(requestList, anno)
				}
			}

			if len(requestList) == 0 || !util.Contains(requestList, requestInstance.Namespace+"."+requestInstance.Name+"."+operand.Name+"/request") {
				klog.Infof("Subscription %s in the namespace %s is NOT managed by %s/%s, Skip reconciling Operands", sub.Name, sub.Namespace, requestInstance.Namespace, requestInstance.Name)
				requestInstance.SetMemberStatus(operand.Name, "", operatorv1alpha1.ServiceFailed, &r.Mutex)
				continue
			}

			klog.V(3).Info("Generating customresource base on ClusterServiceVersion: ", csv.GetName())
			requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorRunning, "", &r.Mutex)

			// Merge and Generate CR
			if operand.Kind == "" {
				configInstance, err := r.GetOperandConfig(ctx, registryKey)
				if err == nil {
					// Check the requested Service Config if exist in specific OperandConfig
					opdConfig := configInstance.GetService(operand.Name)
					if opdConfig == nil {
						klog.V(2).Infof("There is no service: %s from the OperandConfig instance: %s/%s, Skip reconciling Operands", operand.Name, registryKey.Namespace, req.Registry)
						continue
					}
					err = r.reconcileCRwithConfig(ctx, opdConfig, configInstance.Name, configInstance.Namespace, csv, requestInstance, operand.Name, sub.Namespace, &r.Mutex)
					if err != nil {
						merr.Add(err)
						requestInstance.SetMemberStatus(operand.Name, "", operatorv1alpha1.ServiceFailed, &r.Mutex)
						continue
					}
				} else if apierrors.IsNotFound(err) {
					klog.Infof("Not Found OperandConfig: %s/%s", operand.Name, err)
				} else {
					merr.Add(errors.Wrapf(err, "failed to get the OperandConfig %s", registryKey.String()))
					continue
				}

			} else {
				err = r.reconcileCRwithRequest(ctx, requestInstance, operand, types.NamespacedName{Name: requestInstance.Name, Namespace: requestInstance.Namespace}, i, sub.Namespace, &r.Mutex)
				if err != nil {
					merr.Add(err)
					requestInstance.SetMemberStatus(operand.Name, "", operatorv1alpha1.ServiceFailed, &r.Mutex)
					continue
				}
			}
			requestInstance.SetMemberStatus(operand.Name, "", operatorv1alpha1.ServiceRunning, &r.Mutex)
		}
	}
	if len(merr.Errors) != 0 {
		return merr
	}
	klog.V(1).Infof("Finished reconciling Operands for OperandRequest: %s/%s", requestInstance.GetNamespace(), requestInstance.GetName())
	return &util.MultiErr{}
}

// reconcileCRwithConfig merge and create custom resource base on OperandConfig and CSV alm-examples
func (r *Reconciler) reconcileCRwithConfig(ctx context.Context, service *operatorv1alpha1.ConfigService, opConfigName, opConfigNs string, csv *olmv1alpha1.ClusterServiceVersion, requestInstance *operatorv1alpha1.OperandRequest, operandName string, operatorNamespace string, mu sync.Locker) error {
	merr := &util.MultiErr{}

	// Create k8s resources required by service
	if service.Resources != nil {
		for _, res := range service.Resources {
			if res.APIVersion == "" {
				return fmt.Errorf("The APIVersion of k8s resource is empty for operator " + service.Name)
			}

			if res.Kind == "" {
				return fmt.Errorf("The Kind of k8s resource is empty for operator " + service.Name)
			}
			if res.Name == "" {
				return fmt.Errorf("The Name of k8s resource is empty for operator " + service.Name)
			}
			var k8sResNs string
			if res.Namespace == "" {
				k8sResNs = opConfigNs
			} else {
				k8sResNs = res.Namespace
			}

			resObject, err := util.ObjectToNewUnstructured(&res)
			if err != nil {
				klog.Errorf("Failed to convert %s %s/%s object to unstructured.Unstructured object", res.Kind, k8sResNs, res.Name)
				return err
			}

			if err := r.ParseValueReferenceInObject(ctx, "data", resObject.Object["data"], resObject.Object, "OperandConfig", opConfigName, opConfigNs, false); err != nil {
				klog.Errorf("Failed to parse value reference in resource %s/%s: %v", k8sResNs, res.Name, err)
				return err
			}
			// cover unstructured.Unstructured object to original OperandConfig object
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(resObject.Object, &res); err != nil {
				klog.Errorf("Failed to convert unstructured.Unstructured object to %s %s/%s object", res.Kind, k8sResNs, res.Name)
				return err
			}

			var k8sRes unstructured.Unstructured
			k8sRes.SetAPIVersion(res.APIVersion)
			k8sRes.SetKind(res.Kind)
			k8sRes.SetName(res.Name)
			k8sRes.SetNamespace(k8sResNs)

			verbs := []string{"create", "delete", "get", "update"}
			if r.checkResAuth(ctx, verbs, k8sRes) {
				err := r.Client.Get(ctx, types.NamespacedName{
					Name:      res.Name,
					Namespace: k8sResNs,
				}, &k8sRes)

				if err != nil && !apierrors.IsNotFound(err) {
					merr.Add(errors.Wrapf(err, "failed to get k8s resource %s/%s", k8sResNs, res.Name))
				} else if apierrors.IsNotFound(err) {
					if err := r.createK8sResource(ctx, k8sRes, res.Data, res.Labels, res.Annotations); err != nil {
						merr.Add(err)
					}
				} else {
					if r.CheckLabel(k8sRes, map[string]string{constant.OpreqLabel: "true"}) && res.Force {
						// Update k8s resource
						klog.V(3).Info("Found existing k8s resource: " + res.Name)
						if err := r.updateK8sResource(ctx, k8sRes, res.Data, res.Labels, res.Annotations); err != nil {
							merr.Add(err)
						}
					} else {
						klog.V(2).Infof("Skip the k8s resource %s/%s which is not created by ODLM", res.Kind, res.Name)
					}
				}
			} else {
				klog.Infof("ODLM doesn't have enough permission to reconcile k8s resource -- Kind: %s, NamespacedName: %s/%s", res.Kind, k8sResNs, res.Name)
			}
		}

		if len(merr.Errors) != 0 {
			return merr
		}
	}

	almExamples := csv.GetAnnotations()["alm-examples"]

	// Convert CR template string to slice
	var almExampleList []interface{}
	err := json.Unmarshal([]byte(almExamples), &almExampleList)
	if err != nil {
		return errors.Wrapf(err, "failed to convert alm-examples in the Subscription %s/%s to slice", opConfigNs, service.Name)
	}

	foundMap := make(map[string]bool)
	for cr := range service.Spec {
		foundMap[cr] = false
	}

	serviceObject, err := util.ObjectToNewUnstructured(service)
	if err != nil {
		klog.Errorf("Failed to convert OperandConfig service object %s to unstructured.Unstructured object", service.Name)
		return err
	}

	if err := r.ParseValueReferenceInObject(ctx, "spec", serviceObject.Object["spec"], serviceObject.Object, "OperandConfig", opConfigName, opConfigNs, false); err != nil {
		klog.Errorf("Failed to parse value reference for service %s: %v", service.Name, err)
		return err
	}
	// cover unstructured.Unstructured object to original OperandConfig object
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(serviceObject.Object, service); err != nil {
		klog.Errorf("Failed to convert unstructured.Unstructured object to service object %s", service.Name)
		return err
	}

	// Merge OperandConfig and ClusterServiceVersion alm-examples
	for _, almExample := range almExampleList {
		// Create an unstructured object for CR and check its value
		var crFromALM unstructured.Unstructured
		crFromALM.Object = almExample.(map[string]interface{})

		name := crFromALM.GetName()
		spec := crFromALM.Object["spec"]
		if spec == nil {
			continue
		}

		err := r.Client.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: opConfigNs,
		}, &crFromALM)

		foundInConfig := false
		for cr := range service.Spec {
			if strings.EqualFold(crFromALM.GetKind(), cr) {
				foundMap[cr] = true
				foundInConfig = true
			}
		}

		if !foundInConfig {
			klog.Warningf("%v in the alm-example doesn't exist in the OperandConfig for %v", crFromALM.GetKind(), csv.GetName())
			continue
		}

		if err != nil && !apierrors.IsNotFound(err) {
			merr.Add(errors.Wrapf(err, "failed to get the custom resource %s/%s", opConfigNs, name))
			continue
		} else if apierrors.IsNotFound(err) {
			// Create Custom Resource
			if err := r.compareConfigandExample(ctx, crFromALM, service, opConfigNs); err != nil {
				merr.Add(err)
				continue
			}
		} else {
			if r.CheckLabel(crFromALM, map[string]string{constant.OpreqLabel: "true"}) {
				// Update or Delete Custom Resource
				if err := r.existingCustomResource(ctx, crFromALM, spec.(map[string]interface{}), service, opConfigNs); err != nil {
					merr.Add(err)
					continue
				}
				statusSpec, err := r.getOperandStatus(crFromALM)
				if err != nil {
					return err
				}
				serviceKind := crFromALM.GetKind()
				if serviceKind != "OperandRequest" && statusSpec.ObjectName != "" {
					var resources []operatorv1alpha1.OperandStatus
					resources = append(resources, statusSpec)
					serviceSpec := newServiceStatus(operandName, operatorNamespace, resources)
					seterr := requestInstance.SetServiceStatus(ctx, serviceSpec, r.Client, mu)
					if seterr != nil {
						return seterr
					}
				}
			} else {
				klog.V(2).Info("Skip the custom resource not created by ODLM")
			}
		}
	}
	if len(merr.Errors) != 0 {
		return merr
	}

	for cr, found := range foundMap {
		if !found {
			klog.Warningf("Custom resource %v doesn't exist in the alm-example of %v", cr, csv.GetName())
		}
	}

	return nil
}

// reconcileCRwithRequest merge and create custom resource base on OperandRequest and CSV alm-examples
func (r *Reconciler) reconcileCRwithRequest(ctx context.Context, requestInstance *operatorv1alpha1.OperandRequest, operand operatorv1alpha1.Operand, requestKey types.NamespacedName, index int, operatorNamespace string, mu sync.Locker) error {
	merr := &util.MultiErr{}

	// Create an unstructured object for CR and check its value
	var crFromRequest unstructured.Unstructured

	if operand.APIVersion == "" {
		return fmt.Errorf("The APIVersion of operand is empty for operator " + operand.Name)
	}

	if operand.Kind == "" {
		return fmt.Errorf("The Kind of operand is empty for operator " + operand.Name)
	}

	var name string
	if operand.InstanceName == "" {
		crInfo := sha256.Sum256([]byte(operand.APIVersion + operand.Kind + strconv.Itoa(index)))
		name = requestKey.Name + "-" + hex.EncodeToString(crInfo[:7])
	} else {
		name = operand.InstanceName
	}

	crFromRequest.SetName(name)
	crFromRequest.SetNamespace(requestKey.Namespace)
	crFromRequest.SetAPIVersion(operand.APIVersion)
	crFromRequest.SetKind(operand.Kind)
	// Set the OperandRequest as the controller of the CR from request
	if err := controllerutil.SetOwnerReference(requestInstance, &crFromRequest, r.Scheme); err != nil {
		merr.Add(errors.Wrapf(err, "failed to set ownerReference for custom resource %s/%s", requestKey.Namespace, name))
	}

	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: requestKey.Namespace,
	}, &crFromRequest)

	if err != nil && !apierrors.IsNotFound(err) {
		merr.Add(errors.Wrapf(err, "failed to get custom resource %s/%s", requestKey.Namespace, name))
	} else if apierrors.IsNotFound(err) {
		// Create Custom resource
		if err := r.createCustomResource(ctx, crFromRequest, requestKey.Namespace, operand.Kind, operand.Spec.Raw); err != nil {
			merr.Add(err)
		}
		requestInstance.SetMemberCRStatus(operand.Name, name, operand.Kind, operand.APIVersion, &r.Mutex)
	} else {
		if r.CheckLabel(crFromRequest, map[string]string{constant.OpreqLabel: "true"}) {
			// Update or Delete Custom resource
			klog.V(3).Info("Found existing custom resource: " + operand.Kind)
			if err := r.updateCustomResource(ctx, crFromRequest, requestKey.Namespace, operand.Kind, operand.Spec.Raw, map[string]interface{}{}, requestInstance); err != nil {
				return err
			}
			statusSpec, err := r.getOperandStatus(crFromRequest)
			if err != nil {
				return err
			}
			if operand.Kind != "OperandRequest" && statusSpec.ObjectName != "" {
				var resources []operatorv1alpha1.OperandStatus
				resources = append(resources, statusSpec)
				serviceSpec := newServiceStatus(operand.Name, operatorNamespace, resources)
				seterr := requestInstance.SetServiceStatus(ctx, serviceSpec, r.Client, mu)
				if seterr != nil {
					return seterr
				}
			}
		} else {
			klog.V(2).Info("Skip the custom resource not created by ODLM")
		}
	}

	if len(merr.Errors) != 0 {
		return merr
	}
	return nil
}

func (r *Reconciler) getOperandStatus(existingCR unstructured.Unstructured) (operatorv1alpha1.OperandStatus, error) {
	var emptyStatus operatorv1alpha1.OperandStatus
	byteStatus, err := json.Marshal(existingCR.Object["status"])
	if err != nil {
		klog.Error(err)
		return emptyStatus, err
	}
	var rawStatus map[string]interface{}
	err = json.Unmarshal(byteStatus, &rawStatus)
	if err != nil {
		klog.Error(err)
		return emptyStatus, err
	}
	var serviceStatus operatorv1alpha1.OperandStatus
	byteService, err := json.Marshal(rawStatus["service"])
	if err != nil {
		klog.Error(err)
		return emptyStatus, err
	}
	err = json.Unmarshal(byteService, &serviceStatus)
	if err != nil {
		klog.Error(err)
		return emptyStatus, err
	}
	return serviceStatus, nil
}

func newServiceStatus(operatorName string, namespace string, resources []operatorv1alpha1.OperandStatus) operatorv1alpha1.ServiceStatus {
	var serviceSpec operatorv1alpha1.ServiceStatus
	serviceSpec.OperatorName = operatorName
	serviceSpec.Namespace = namespace
	// serviceSpec.Type = "Ready" //should this be something more specific? Like operandNameReady?
	status := "Ready"
	for i := range resources {
		if resources[i].Status == "NotReady" {
			status = "NotReady"
			break
		} else {
			for j := range resources[i].ManagedResources {
				if resources[i].ManagedResources[j].Status == "NotReady" {
					status = "NotReady"
					break
				}
			}
		}
	}
	serviceSpec.Status = status //TODO logic to determine readiness
	// serviceSpec.LastUpdateTime = time.Now().Format(time.RFC3339)
	serviceSpec.Resources = resources
	return serviceSpec
}

// deleteAllCustomResource remove custom resource base on OperandConfig and CSV alm-examples
func (r *Reconciler) deleteAllCustomResource(ctx context.Context, csv *olmv1alpha1.ClusterServiceVersion, requestInstance *operatorv1alpha1.OperandRequest, csc *operatorv1alpha1.OperandConfig, operandName, namespace string) error {

	customeResourceMap := make(map[string]operatorv1alpha1.OperandCRMember)
	for _, member := range requestInstance.Status.Members {
		if len(member.OperandCRList) != 0 {
			if member.Name == operandName {
				for _, cr := range member.OperandCRList {
					customeResourceMap[member.Name+"/"+cr.Kind+"/"+cr.Name] = cr
				}
			}
		}
	}

	merr := &util.MultiErr{}
	var (
		wg sync.WaitGroup
	)
	for index, opdMember := range customeResourceMap {
		crShouldBeDeleted := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": opdMember.APIVersion,
				"kind":       opdMember.Kind,
				"metadata": map[string]interface{}{
					"name": opdMember.Name,
				},
			},
		}

		var (
			operatorName = strings.Split(index, "/")[0]
			opdMember    = opdMember
		)

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.deleteCustomResource(ctx, crShouldBeDeleted, requestInstance.Namespace); err != nil {
				r.Mutex.Lock()
				defer r.Mutex.Unlock()
				merr.Add(err)
				return
			}
			requestInstance.RemoveMemberCRStatus(operatorName, opdMember.Name, opdMember.Kind, &r.Mutex)
		}()
	}
	wg.Wait()

	if len(merr.Errors) != 0 {
		return merr
	}

	service := csc.GetService(operandName)
	if service == nil {
		return nil
	}
	almExamples := csv.GetAnnotations()["alm-examples"]
	klog.V(2).Info("Delete all the custom resource from Subscription ", service.Name)

	// Create a slice for crTemplates
	var almExamplesRaw []interface{}

	// Convert CR template string to slice
	err := json.Unmarshal([]byte(almExamples), &almExamplesRaw)
	if err != nil {
		return errors.Wrapf(err, "failed to convert alm-examples in the Subscription %s to slice", service.Name)
	}

	// Merge OperandConfig and ClusterServiceVersion alm-examples
	for _, crFromALM := range almExamplesRaw {

		// Get CR from the alm-example
		var crTemplate unstructured.Unstructured
		crTemplate.Object = crFromALM.(map[string]interface{})
		crTemplate.SetNamespace(namespace)
		name := crTemplate.GetName()
		// Get the kind of CR
		kind := crTemplate.GetKind()
		// Delete the CR
		for crdName := range service.Spec {

			// Compare the name of OperandConfig and CRD name
			if strings.EqualFold(kind, crdName) {
				err := r.Client.Get(ctx, types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				}, &crTemplate)
				if err != nil && !apierrors.IsNotFound(err) {
					merr.Add(err)
					continue
				}
				if apierrors.IsNotFound(err) {
					klog.V(2).Info("Finish Deleting the CR: " + kind)
					continue
				}
				if r.CheckLabel(crTemplate, map[string]string{constant.OpreqLabel: "true"}) {
					wg.Add(1)
					go func() {
						defer wg.Done()
						if err := r.deleteCustomResource(ctx, crTemplate, namespace); err != nil {
							r.Mutex.Lock()
							defer r.Mutex.Unlock()
							merr.Add(err)
						}
					}()
				}

			}

		}
	}
	wg.Wait()
	if len(merr.Errors) != 0 {
		return merr
	}

	return nil
}

func (r *Reconciler) compareConfigandExample(ctx context.Context, crTemplate unstructured.Unstructured, service *operatorv1alpha1.ConfigService, namespace string) error {
	kind := crTemplate.GetKind()

	for crdName, crdConfig := range service.Spec {
		// Compare the name of OperandConfig and CRD name
		if strings.EqualFold(kind, crdName) {
			klog.V(3).Info("Found OperandConfig spec for custom resource: " + kind)
			err := r.createCustomResource(ctx, crTemplate, namespace, crdName, crdConfig.Raw)
			if err != nil {
				return errors.Wrapf(err, "failed to create custom resource -- Kind: %s", kind)
			}
		}
	}
	return nil
}

func (r *Reconciler) createCustomResource(ctx context.Context, crTemplate unstructured.Unstructured, namespace, crName string, crConfig []byte) error {

	//Convert CR template spec to string
	specJSONString, _ := json.Marshal(crTemplate.Object["spec"])

	// Merge CR template spec and OperandConfig spec
	mergedCR := util.MergeCR(specJSONString, crConfig)

	crTemplate.Object["spec"] = mergedCR
	crTemplate.SetNamespace(namespace)

	r.EnsureLabel(crTemplate, map[string]string{constant.OpreqLabel: "true"})

	// Create the CR
	crerr := r.Create(ctx, &crTemplate)
	if crerr != nil && !apierrors.IsAlreadyExists(crerr) {
		return errors.Wrap(crerr, "failed to create custom resource")
	}

	klog.V(2).Info("Finish creating the Custom Resource: ", crName)

	return nil
}

func (r *Reconciler) existingCustomResource(ctx context.Context, existingCR unstructured.Unstructured, specFromALM map[string]interface{}, service *operatorv1alpha1.ConfigService, namespace string) error {
	kind := existingCR.GetKind()

	var found bool
	for crName, crdConfig := range service.Spec {
		// Compare the name of OperandConfig and CRD name
		if strings.EqualFold(kind, crName) {
			found = true
			klog.V(3).Info("Found OperandConfig spec for custom resource: " + kind)
			err := r.updateCustomResource(ctx, existingCR, namespace, crName, crdConfig.Raw, specFromALM)
			if err != nil {
				return errors.Wrap(err, "failed to update custom resource")
			}
		}
	}
	if !found {
		err := r.deleteCustomResource(ctx, existingCR, namespace)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) updateCustomResource(ctx context.Context, existingCR unstructured.Unstructured, namespace, crName string, crConfig []byte, configFromALM map[string]interface{}, owners ...metav1.Object) error {

	kind := existingCR.GetKind()
	apiversion := existingCR.GetAPIVersion()
	name := existingCR.GetName()
	// Update the CR
	err := wait.PollImmediate(constant.DefaultCRFetchPeriod, constant.DefaultCRFetchTimeout, func() (bool, error) {

		existingCR := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": apiversion,
				"kind":       kind,
			},
		}

		err := r.Client.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, &existingCR)

		if err != nil {
			return false, errors.Wrapf(err, "failed to get custom resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
		}

		if !r.CheckLabel(existingCR, map[string]string{constant.OpreqLabel: "true"}) {
			return true, nil
		}

		forceUpdate := false
		for _, owner := range owners {
			if err := controllerutil.SetOwnerReference(owner, &existingCR, r.Scheme); err != nil {
				return false, errors.Wrapf(err, "failed to set ownerReference for custom resource %s/%s", existingCR.GetNamespace(), existingCR.GetName())
			}
			forceUpdate = true
		}

		configFromALMRaw, err := json.Marshal(configFromALM)
		if err != nil {
			klog.Error(err)
			return false, err
		}

		existingCRRaw, err := json.Marshal(existingCR.Object["spec"])
		if err != nil {
			klog.Error(err)
			return false, err
		}

		// Merge spec from ALM example and existing CR
		updatedExistingCR := util.MergeCR(configFromALMRaw, existingCRRaw)

		updatedExistingCRRaw, err := json.Marshal(updatedExistingCR)
		if err != nil {
			klog.Error(err)
			return false, err
		}

		// Merge spec from update existing CR and OperandConfig spec
		updatedCRSpec := util.MergeCR(updatedExistingCRRaw, crConfig)

		CRgeneration := existingCR.GetGeneration()

		if reflect.DeepEqual(existingCR.Object["spec"], updatedCRSpec) && !forceUpdate {
			return true, nil
		}

		klog.V(2).Infof("updating custom resource with apiversion: %s, kind: %s, %s/%s", apiversion, kind, namespace, name)

		existingCR.Object["spec"] = updatedCRSpec
		err = r.Update(ctx, &existingCR)

		if err != nil {
			return false, errors.Wrapf(err, "failed to update custom resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
		}

		UpdatedCR := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": apiversion,
				"kind":       kind,
			},
		}

		err = r.Client.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, &UpdatedCR)

		if err != nil {
			return false, errors.Wrapf(err, "failed to get custom resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)

		}

		if UpdatedCR.GetGeneration() != CRgeneration {
			klog.V(2).Info("Finish updating the Custom Resource: ", crName)
		}

		return true, nil
	})

	if err != nil {
		return errors.Wrapf(err, "failed to update custom resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
	}

	return nil
}

func (r *Reconciler) deleteCustomResource(ctx context.Context, existingCR unstructured.Unstructured, namespace string) error {

	kind := existingCR.GetKind()
	apiversion := existingCR.GetAPIVersion()
	name := existingCR.GetName()

	crShouldBeDeleted := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiversion,
			"kind":       kind,
		},
	}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &crShouldBeDeleted)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to get custom resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
	}
	if apierrors.IsNotFound(err) {
		klog.V(3).Infof("There is no custom resource: %s from custom resource definition: %s", name, kind)
	} else {
		if r.CheckLabel(crShouldBeDeleted, map[string]string{constant.OpreqLabel: "true"}) && !r.CheckLabel(crShouldBeDeleted, map[string]string{constant.NotUninstallLabel: "true"}) {
			klog.V(3).Infof("Deleting custom resource: %s from custom resource definition: %s", name, kind)
			err := r.Delete(ctx, &crShouldBeDeleted)
			if err != nil && !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to delete custom resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
			}
			err = wait.PollImmediate(constant.DefaultCRDeletePeriod, constant.DefaultCRDeleteTimeout, func() (bool, error) {
				if strings.EqualFold(kind, "OperandRequest") {
					return true, nil
				}
				klog.V(3).Infof("Waiting for CR %s is removed ...", kind)
				err := r.Client.Get(ctx, types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				}, &existingCR)
				if apierrors.IsNotFound(err) {
					return true, nil
				}
				if err != nil {
					return false, errors.Wrapf(err, "failed to get custom resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
				}
				return false, nil
			})
			if err != nil {
				return errors.Wrapf(err, "failed to delete custom resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
			}
			klog.V(1).Infof("Finish deleting custom resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
		}
	}
	return nil
}

func (r *Reconciler) checkCustomResource(ctx context.Context, requestInstance *operatorv1alpha1.OperandRequest) error {
	klog.V(3).Infof("deleting the custom resource from OperandRequest %s/%s", requestInstance.Namespace, requestInstance.Name)

	members := requestInstance.Status.Members

	customeResourceMap := make(map[string]operatorv1alpha1.OperandCRMember)
	for _, member := range members {
		if len(member.OperandCRList) != 0 {
			for _, cr := range member.OperandCRList {
				customeResourceMap[member.Name+"/"+cr.Kind+"/"+cr.Name] = cr
			}
		}
	}
	for _, req := range requestInstance.Spec.Requests {
		for _, opd := range req.Operands {
			if opd.Kind != "" {
				var name string
				if opd.InstanceName == "" {
					name = requestInstance.Name
				} else {
					name = opd.InstanceName
				}
				delete(customeResourceMap, opd.Name+"/"+opd.Kind+"/"+name)
			}
		}
	}

	var (
		wg sync.WaitGroup
	)

	merr := &util.MultiErr{}
	for index, opdMember := range customeResourceMap {
		crShouldBeDeleted := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": opdMember.APIVersion,
				"kind":       opdMember.Kind,
				"metadata": map[string]interface{}{
					"name": opdMember.Name,
				},
			},
		}

		var (
			operatorName = strings.Split(index, "/")[0]
			opdMember    = opdMember
		)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.deleteCustomResource(ctx, crShouldBeDeleted, requestInstance.Namespace); err != nil {
				r.Mutex.Lock()
				defer r.Mutex.Unlock()
				merr.Add(err)
				return
			}
			requestInstance.RemoveMemberCRStatus(operatorName, opdMember.Name, opdMember.Kind, &r.Mutex)
		}()
	}
	wg.Wait()

	if len(merr.Errors) != 0 {
		return merr
	}

	return nil
}

func (r *Reconciler) createK8sResource(ctx context.Context, k8sResTemplate unstructured.Unstructured, k8sResConfig *runtime.RawExtension, newLabels, newAnnotations map[string]string) error {
	kind := k8sResTemplate.GetKind()
	name := k8sResTemplate.GetName()
	namespace := k8sResTemplate.GetNamespace()

	if k8sResConfig != nil {
		k8sResConfigDecoded := make(map[string]interface{})
		k8sResConfigUnmarshalErr := json.Unmarshal(k8sResConfig.Raw, &k8sResConfigDecoded)
		if k8sResConfigUnmarshalErr != nil {
			klog.Errorf("failed to unmarshal k8s Resource Config: %v", k8sResConfigUnmarshalErr)
		}

		for k, v := range k8sResConfigDecoded {
			k8sResTemplate.Object[k] = v
		}
	}

	r.EnsureLabel(k8sResTemplate, map[string]string{constant.OpreqLabel: "true"})
	r.EnsureLabel(k8sResTemplate, newLabels)
	r.EnsureAnnotation(k8sResTemplate, newAnnotations)

	// Create the k8s resource
	err := r.Create(ctx, &k8sResTemplate)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create k8s resource")
	}

	klog.V(2).Infof("Finish creating the k8s Resource: -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)

	return nil
}

func (r *Reconciler) updateK8sResource(ctx context.Context, existingK8sRes unstructured.Unstructured, k8sResConfig *runtime.RawExtension, newLabels, newAnnotations map[string]string) error {
	kind := existingK8sRes.GetKind()
	apiversion := existingK8sRes.GetAPIVersion()
	name := existingK8sRes.GetName()
	namespace := existingK8sRes.GetNamespace()
	if kind == "Job" {
		existingK8sRes := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": apiversion,
				"kind":       kind,
			},
		}

		err := r.Client.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, &existingK8sRes)

		if err != nil {
			return errors.Wrapf(err, "failed to get k8s resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
		}
		if !r.CheckLabel(existingK8sRes, map[string]string{constant.OpreqLabel: "true"}) {
			return nil
		}

		var existingHashedData string
		var newHashedData string
		if existingK8sRes.GetAnnotations() != nil {
			existingHashedData = existingK8sRes.GetAnnotations()[constant.HashedData]
		}

		if k8sResConfig != nil {
			hashedData := sha256.Sum256(k8sResConfig.Raw)
			newHashedData = hex.EncodeToString(hashedData[:7])
		}

		if existingHashedData != newHashedData {
			// create a new template of k8s resource
			var templatek8sRes unstructured.Unstructured
			templatek8sRes.SetAPIVersion(apiversion)
			templatek8sRes.SetKind(kind)
			templatek8sRes.SetName(name)
			templatek8sRes.SetNamespace(namespace)

			if newAnnotations == nil {
				newAnnotations = make(map[string]string)
			}
			newAnnotations[constant.HashedData] = newHashedData

			if err := r.deleteK8sResource(ctx, existingK8sRes, namespace); err != nil {
				return errors.Wrap(err, "failed to update k8s resource")
			}
			if err := r.createK8sResource(ctx, templatek8sRes, k8sResConfig, newLabels, newAnnotations); err != nil {
				return errors.Wrap(err, "failed to update k8s resource")
			}
		}

		return nil
	}

	// Update the k8s res
	err := wait.PollImmediate(constant.DefaultCRFetchPeriod, constant.DefaultCRFetchTimeout, func() (bool, error) {

		existingK8sRes := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": apiversion,
				"kind":       kind,
			},
		}

		err := r.Client.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, &existingK8sRes)

		if err != nil {
			return false, errors.Wrapf(err, "failed to get k8s resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
		}

		if !r.CheckLabel(existingK8sRes, map[string]string{constant.OpreqLabel: "true"}) {
			return true, nil
		}

		// isEqual := r.CheckAnnotation(existingK8sRes, newAnnotations) && r.CheckLabel(existingK8sRes, newLabels)
		if k8sResConfig != nil {
			k8sResConfigDecoded := make(map[string]interface{})
			k8sResConfigUnmarshalErr := json.Unmarshal(k8sResConfig.Raw, &k8sResConfigDecoded)
			if k8sResConfigUnmarshalErr != nil {
				klog.Errorf("failed to unmarshal k8s Resource Config: %v", k8sResConfigUnmarshalErr)
			}

			for k, v := range k8sResConfigDecoded {
				// isEqual = isEqual && reflect.DeepEqual(existingK8sRes.Object[k], v)
				existingK8sRes.Object[k] = v
			}
		}

		CRgeneration := existingK8sRes.GetGeneration()

		// if isEqual {
		// 	return true, nil
		// }

		r.EnsureAnnotation(existingK8sRes, newAnnotations)
		r.EnsureLabel(existingK8sRes, newLabels)

		klog.V(2).Infof("updating k8s resource with apiversion: %s, kind: %s, %s/%s", apiversion, kind, namespace, name)

		err = r.Update(ctx, &existingK8sRes)

		if err != nil {
			return false, errors.Wrapf(err, "failed to update k8s resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
		}

		UpdatedK8sRes := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": apiversion,
				"kind":       kind,
			},
		}

		err = r.Client.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, &UpdatedK8sRes)

		if err != nil {
			return false, errors.Wrapf(err, "failed to get k8s resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)

		}

		if UpdatedK8sRes.GetGeneration() != CRgeneration {
			klog.V(2).Infof("Finish updating the k8s Resource: -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
		}

		return true, nil
	})

	if err != nil {
		return errors.Wrapf(err, "failed to update k8s resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
	}

	return nil
}

func (r *Reconciler) deleteK8sResource(ctx context.Context, existingK8sRes unstructured.Unstructured, namespace string) error {

	kind := existingK8sRes.GetKind()
	apiversion := existingK8sRes.GetAPIVersion()
	name := existingK8sRes.GetName()

	k8sResShouldBeDeleted := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiversion,
			"kind":       kind,
		},
	}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &k8sResShouldBeDeleted)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to get k8s resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
	}
	if apierrors.IsNotFound(err) {
		klog.V(3).Infof("There is no k8s resource: %s from kind: %s", name, kind)
	} else {
		if r.CheckLabel(k8sResShouldBeDeleted, map[string]string{constant.OpreqLabel: "true"}) && !r.CheckLabel(k8sResShouldBeDeleted, map[string]string{constant.NotUninstallLabel: "true"}) {
			klog.V(3).Infof("Deleting k8s resource: %s from kind: %s", name, kind)
			err := r.Delete(ctx, &k8sResShouldBeDeleted, client.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil && !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to delete k8s resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
			}
			err = wait.PollImmediate(constant.DefaultCRDeletePeriod, constant.DefaultCRDeleteTimeout, func() (bool, error) {
				klog.V(3).Infof("Waiting for k8s resource %s is removed ...", kind)
				err := r.Client.Get(ctx, types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				}, &existingK8sRes)
				if apierrors.IsNotFound(err) {
					return true, nil
				}
				if err != nil {
					return false, errors.Wrapf(err, "failed to get k8s resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
				}
				return false, nil
			})
			if err != nil {
				return errors.Wrapf(err, "failed to delete k8s resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
			}
			klog.V(1).Infof("Finish deleting k8s resource -- Kind: %s, NamespacedName: %s/%s", kind, namespace, name)
		}
	}
	return nil
}

// deleteAllK8sResource remove k8s resource base on OperandConfig
func (r *Reconciler) deleteAllK8sResource(ctx context.Context, csc *operatorv1alpha1.OperandConfig, operandName, namespace string) error {

	service := csc.GetService(operandName)
	if service == nil {
		return nil
	}

	var k8sResourceList []operatorv1alpha1.ConfigResource
	k8sResourceList = append(k8sResourceList, service.Resources...)

	merr := &util.MultiErr{}
	var (
		wg sync.WaitGroup
	)
	for _, k8sRes := range k8sResourceList {
		k8sResShouldBeDeleted := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": k8sRes.APIVersion,
				"kind":       k8sRes.Kind,
				"metadata": map[string]interface{}{
					"name": k8sRes.Name,
				},
			},
		}
		k8sNamespace := namespace
		if k8sRes.Namespace != "" {
			k8sNamespace = k8sRes.Namespace
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.deleteK8sResource(ctx, k8sResShouldBeDeleted, k8sNamespace); err != nil {
				r.Mutex.Lock()
				defer r.Mutex.Unlock()
				merr.Add(err)
				return
			}
		}()
	}
	wg.Wait()

	if len(merr.Errors) != 0 {
		return merr
	}
	return nil
}

func (r *Reconciler) checkResAuth(ctx context.Context, verbs []string, k8sResTemplate unstructured.Unstructured) bool {
	kind := k8sResTemplate.GetKind()
	apiversion := k8sResTemplate.GetAPIVersion()
	name := k8sResTemplate.GetName()
	namespace := k8sResTemplate.GetNamespace()

	dc := discovery.NewDiscoveryClientForConfigOrDie(r.Config)
	if namespaced, err := util.ResourceNamespaced(dc, apiversion, kind); err != nil {
		klog.Errorf("Failed to check resource scope for Kind: %s, NamespacedName: %s/%s, %v", kind, namespace, name, err)
	} else if !namespaced {
		namespace = ""
	}

	gvk := schema.FromAPIVersionAndKind(apiversion, kind)
	gvr, err := r.ResourceForKind(gvk, namespace)
	if err != nil {
		klog.Errorf("Failed to get GroupVersionResource from GroupVersionKind, %v", err)
		return false
	}

	for _, verb := range verbs {
		sar := &authorizationv1.SelfSubjectAccessReview{
			Spec: authorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Namespace: namespace,
					Verb:      verb,
					Group:     gvr.Group,
					Resource:  gvr.Resource,
				},
			},
		}
		if err := r.Create(ctx, sar); err != nil {
			klog.Errorf("Failed to check operator permission for Kind: %s, NamespacedName: %s/%s, %v", kind, namespace, name, err)
			return false
		}

		klog.V(2).Infof("Operator %s permission in namespace %s for Kind: %s, Allowed: %t, Denied: %t, Reason: %s", verb, namespace, kind, sar.Status.Allowed, sar.Status.Denied, sar.Status.Reason)

		if !sar.Status.Allowed {
			return false
		}
	}
	return true
}

func (r *Reconciler) ResourceForKind(gvk schema.GroupVersionKind, namespace string) (*schema.GroupVersionResource, error) {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{gvk.GroupVersion()})

	if namespace != "" {
		mapper.Add(gvk, meta.RESTScopeRoot)
	} else {
		mapper.Add(gvk, meta.RESTScopeNamespace)
	}

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}
	return &mapping.Resource, nil
}
