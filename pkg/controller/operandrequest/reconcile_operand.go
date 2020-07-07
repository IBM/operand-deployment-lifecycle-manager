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

package operandrequest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	constant "github.com/IBM/operand-deployment-lifecycle-manager/pkg/constant"
	fetch "github.com/IBM/operand-deployment-lifecycle-manager/pkg/controller/common"
	util "github.com/IBM/operand-deployment-lifecycle-manager/pkg/util"
)

func (r *ReconcileOperandRequest) reconcileOperand(requestKey types.NamespacedName) *util.MultiErr {
	klog.V(1).Infof("Reconciling Operands for OperandRequest %s", requestKey)
	merr := &util.MultiErr{}
	requestInstance, err := fetch.FetchOperandRequest(r.client, requestKey)
	if err != nil {
		merr.Add(err)
		return merr
	}
	// Update request status
	defer func() {
		requestInstance.UpdateClusterPhase()
		if err := r.client.Status().Update(context.TODO(), requestInstance); err != nil {
			klog.Error("Update request status failed: ", err)
		}
	}()

	for _, req := range requestInstance.Spec.Requests {
		registryKey := requestInstance.GetRegistryKey(req)
		configInstance, err := fetch.FetchOperandConfig(r.client, registryKey)
		if err != nil {
			klog.Error("Failed to get the operandconfig instance: ", err)
			merr.Add(err)
			continue
		}
		registryInstance, err := fetch.FetchOperandRegistry(r.client, registryKey)
		if err != nil {
			klog.Error("Failed to get the operandregistry instance: ", err)
			merr.Add(err)
			continue
		}
		for _, operand := range req.Operands {

			// Check the requested Service Config if exist in specific OperandConfig
			opdConfig := configInstance.GetService(operand.Name)
			if opdConfig == nil {
				klog.Warningf("Cannot find %s in the operandconfig instance %s in the namespace %s ", operand.Name, req.Registry, req.RegistryNamespace)
				continue
			}
			opdRegistry := registryInstance.GetOperator(operand.Name)
			if opdRegistry == nil {
				klog.Warningf("Cannot find %s in the operandregistry instance %s in the namespace %s ", operand.Name, req.Registry, req.RegistryNamespace)
				continue
			}

			klog.V(3).Info("Looking for csv for the operator: ", opdConfig.Name)

			// Looking for the CSV
			namespace := getOperatorNamespace(opdRegistry.InstallMode, opdRegistry.Namespace)
			csv, err := r.getClusterServiceVersion(opdConfig.Name, namespace)

			// If can't get CSV, requeue the request
			if err != nil {
				klog.Errorf("Failed to get the ClusterServiceVersion for the Subscription %s in the namespace %s: %s", opdConfig.Name, namespace, err)
				merr.Add(err)
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorFailed, "")
				continue
			}

			if csv == nil {
				klog.Warningf("ClusterServiceVersion for the Subscription %s in the namespace %s is not ready yet, retry", opdConfig.Name, namespace)
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorInstalling, "")
				continue
			}

			if csv.Status.Phase == olmv1alpha1.CSVPhaseFailed {
				klog.Errorf("The ClusterServiceVersion for Subscription %s is Failed", opdConfig.Name)
				merr.Add(fmt.Errorf("the ClusterServiceVersion for Subscription %s is Failed", opdConfig.Name))
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorFailed, "")
				continue
			}
			if csv.Status.Phase != olmv1alpha1.CSVPhaseSucceeded {
				klog.Errorf("The ClusterServiceVersion for Subscription %s is not Ready", opdConfig.Name)
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorInstalling, "")
			}

			klog.V(3).Info("Generating customresource base on ClusterServiceVersion: ", csv.ObjectMeta.Name)
			requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorRunning, "")

			// Merge and Generate CR
			err = r.reconcileCr(opdConfig, opdRegistry.Namespace, csv, configInstance)
			if err != nil {
				klog.Error("Failed to get create or update customresource: ", err)
				merr.Add(err)
				requestInstance.SetMemberStatus(operand.Name, "", operatorv1alpha1.ServiceFailed)
			}
			requestInstance.SetMemberStatus(operand.Name, "", operatorv1alpha1.ServiceRunning)
		}
	}
	if len(merr.Errors) != 0 {
		return merr
	}
	klog.V(1).Infof("Finished reconciling Operands for OperandRequest %s", requestKey)
	return &util.MultiErr{}
}

// getCSV retrieves the ClusterServiceVersion
func (r *ReconcileOperandRequest) getClusterServiceVersion(subName, subNamespace string) (*olmv1alpha1.ClusterServiceVersion, error) {
	klog.V(3).Infof("Looking for the ClusterServiceVersion for Subscription %s in the namespace %s", subName, subNamespace)
	sub, err := r.olmClient.OperatorsV1alpha1().Subscriptions(subNamespace).Get(subName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		klog.V(3).Infof("There is no Subscription %s in the namespace %s", subName, subNamespace)
		return nil, nil
	}
	if err != nil {
		klog.Error("Failed to list Subscriptions: ", err)
		return nil, err
	}

	if _, ok := sub.Labels[constant.OpreqLabel]; !ok {
		// Subscription existing and not managed by OperandRequest controller
		klog.V(2).Infof("Subscription %s in the namespace %s isn't created by ODLM. Ignore updating/deleting it", sub.Name, sub.Namespace)
	}

	// Subscription existing and managed by OperandRequest controller

	if sub.Status.CurrentCSV == "" {
		klog.V(3).Infof("The ClusterServiceVersion for Subscription %s is not ready. Will check it again", subName)
		return nil, nil
	}

	csvName := sub.Status.CurrentCSV
	csvNamespace := subNamespace

	if sub.Status.Install == nil || sub.Status.InstallPlanRef.Name == "" {
		klog.V(3).Infof("The Installplan for Subscription %s is not ready. Will check it again", subName)
		return nil, nil
	}

	ipName := sub.Status.InstallPlanRef.Name
	ipNamespace := subNamespace

	// If the installplan is deleted after is completed, ODLM won't block the CR update.
	ip, err := r.olmClient.OperatorsV1alpha1().InstallPlans(ipNamespace).Get(ipName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		klog.Errorf("Failed to get Installplan %s in the namespace %s: %s", ipName, ipNamespace, err)
		return nil, err
	} else if !errors.IsNotFound(err) && ip.Status.Phase == olmv1alpha1.InstallPlanPhaseFailed {
		klog.Errorf("Installplan %s in the namespace %s is failed", ipName, ipNamespace)
		return nil, fmt.Errorf("installplan %s in the namespace %s is failed", ipName, ipNamespace)
	} else if !errors.IsNotFound(err) && ip.Status.Phase != olmv1alpha1.InstallPlanPhaseComplete {
		klog.Infof("Installplan %s in the namespace %s is not ready", ipName, ipNamespace)
		return nil, nil
	}

	csv, getCSVErr := r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(csvNamespace).Get(csvName, metav1.GetOptions{})
	if getCSVErr != nil {
		if errors.IsNotFound(getCSVErr) {
			klog.V(3).Infof("The Subscription %s is upgrading. Will check it when it is stable", subName)
			return nil, nil
		}
		klog.Errorf("Failed to get ClusterServiceVersion %s in the namespace %s: %s", csvName, csvNamespace, getCSVErr)
		return nil, getCSVErr
	}

	klog.V(3).Infof("Get ClusterServiceVersion %s in the namespace %s", csvName, csvNamespace)
	return csv, nil
}

// reconcileCr merge and create custom resource base on OperandConfig and CSV alm-examples
func (r *ReconcileOperandRequest) reconcileCr(service *operatorv1alpha1.ConfigService, namespace string, csv *olmv1alpha1.ClusterServiceVersion, csc *operatorv1alpha1.OperandConfig) error {
	almExamples := csv.ObjectMeta.Annotations["alm-examples"]

	// Create a slice for crTemplates
	var crTemplates []interface{}

	// Convert CR template string to slice
	crTemplatesErr := json.Unmarshal([]byte(almExamples), &crTemplates)
	if crTemplatesErr != nil {
		klog.Errorf("Failed to convert alm-examples in the Subscription %s to slice: %s", service.Name, crTemplatesErr)
		return crTemplatesErr
	}

	merr := &util.MultiErr{}

	// Merge OperandConfig and ClusterServiceVersion alm-examples
	for _, crTemplate := range crTemplates {

		// Create an unstruct object for CR and request its value to CR template
		var unstruct unstructured.Unstructured
		unstruct.Object = crTemplate.(map[string]interface{})

		name := unstruct.Object["metadata"].(map[string]interface{})["name"].(string)

		getError := r.client.Get(context.TODO(), types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, &unstruct)

		if getError != nil && !errors.IsNotFound(getError) {
			klog.Error("Failed to get the custom resource should be deleted with name: ", name, getError)
			merr.Add(getError)
			continue
		} else if errors.IsNotFound(getError) {
			// Create Custom resource
			if createErr := r.compareConfigandExample(unstruct, service, namespace, csc); createErr != nil {
				merr.Add(createErr)
				continue
			}
		} else {
			if checkLabel(unstruct, map[string]string{constant.OpreqLabel: "true"}) {
				// Update or Delete Custom resource
				if updateDeleteErr := r.existingCustomResource(unstruct, service, namespace, csc); updateDeleteErr != nil {
					merr.Add(updateDeleteErr)
					continue
				}
			} else {
				klog.V(2).Info("Skip the custom resource not created by ODLM")
			}
		}
	}
	if len(merr.Errors) != 0 {
		return merr
	}
	return nil
}

// deleteAllCustomResource remove custom resource base on OperandConfig and CSV alm-examples
func (r *ReconcileOperandRequest) deleteAllCustomResource(csv *olmv1alpha1.ClusterServiceVersion, csc *operatorv1alpha1.OperandConfig, operandName, namespace string) error {

	service := csc.GetService(operandName)
	if service == nil {
		return nil
	}
	almExamples := csv.ObjectMeta.Annotations["alm-examples"]
	klog.V(2).Info("Delete all the custom resource from Subscription ", service.Name)

	// Create a slice for crTemplates
	var crTemplates []interface{}

	// Convert CR template string to slice
	crTemplatesErr := json.Unmarshal([]byte(almExamples), &crTemplates)
	if crTemplatesErr != nil {
		klog.Errorf("Failed to convert alm-examples in the Subscription %s to slice: %s", service.Name, crTemplatesErr)
		return crTemplatesErr
	}

	merr := &util.MultiErr{}

	// Merge OperandConfig and ClusterServiceVersion alm-examples
	for _, crTemplate := range crTemplates {

		// Get CR from the alm-example
		var unstruct unstructured.Unstructured
		unstruct.Object = crTemplate.(map[string]interface{})
		unstruct.Object["metadata"].(map[string]interface{})["namespace"] = namespace
		name := unstruct.Object["metadata"].(map[string]interface{})["name"].(string)
		// Get the kind of CR
		kind := unstruct.Object["kind"].(string)
		// Delete the CR
		for crdName := range service.Spec {

			// Compare the name of OperandConfig and CRD name
			if strings.EqualFold(kind, crdName) {
				getError := r.client.Get(context.TODO(), types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				}, &unstruct)
				if getError != nil && !errors.IsNotFound(getError) {
					klog.Errorf("Failed to get the custom resource %s in the namespace %s: %s", name, namespace, getError)
					merr.Add(getError)
					continue
				}
				if errors.IsNotFound(getError) {
					klog.V(2).Info("Finish Deleting the CR: " + kind)
					stateDeleteErr := r.deleteServiceStatus(csc, service.Name, crdName)
					if stateDeleteErr != nil {
						klog.Errorf("Failed to clean up the deleted service status for the custom resource %s in the namespace %s in the operand config: %s", name, namespace, stateDeleteErr)
						merr.Add(stateDeleteErr)
					}
					continue
				}
				if checkLabel(unstruct, map[string]string{constant.OpreqLabel: "true"}) {
					deleteErr := r.deleteCustomResource(unstruct, service, namespace, csc)
					if deleteErr != nil {
						klog.Errorf("Failed to delete custom resource %s in the namespace %s: %s", name, namespace, deleteErr)
						return deleteErr
					}
				}

			}

		}
	}
	if len(merr.Errors) != 0 {
		return merr
	}

	return nil
}

func (r *ReconcileOperandRequest) compareConfigandExample(unstruct unstructured.Unstructured, service *operatorv1alpha1.ConfigService, namespace string, csc *operatorv1alpha1.OperandConfig) error {
	kind := unstruct.Object["kind"].(string)

	for crName, crdConfig := range service.Spec {
		// Compare the name of OperandConfig and CRD name
		if strings.EqualFold(kind, crName) {
			klog.V(3).Info("Found OperandConfig spec for custom resource: " + kind)
			createErr := r.createCustomResource(unstruct, service, namespace, crName, csc, crdConfig.Raw)
			if createErr != nil {
				klog.Error("Failed to create custom resource: ", createErr)
				return createErr
			}
		}
	}
	return nil
}

func (r *ReconcileOperandRequest) createCustomResource(unstruct unstructured.Unstructured, service *operatorv1alpha1.ConfigService, namespace, crName string, csc *operatorv1alpha1.OperandConfig, crConfig []byte) error {

	//Convert CR template spec to string
	specJSONString, _ := json.Marshal(unstruct.Object["spec"])

	// Merge CR template spec and OperandConfig spec
	mergedCR := util.MergeCR(specJSONString, crConfig)

	unstruct.Object["spec"] = mergedCR
	unstruct.Object["metadata"].(map[string]interface{})["namespace"] = namespace

	ensureLabel(unstruct, map[string]string{constant.OpreqLabel: "true"})

	// Creat the CR
	crCreateErr := r.client.Create(context.TODO(), &unstruct)
	if crCreateErr != nil && !errors.IsAlreadyExists(crCreateErr) {
		stateUpdateErr := r.updateServiceStatus(csc, service.Name, crName, operatorv1alpha1.ServiceFailed)
		if stateUpdateErr != nil {
			klog.Error("Failed to update status")
			return stateUpdateErr
		}
		klog.Errorf("Failed to Create the Custom Resource %s: %s ", crName, crCreateErr)
		return crCreateErr
	}

	klog.V(2).Info("Finish creating the Custom Resource: ", crName)
	stateUpdateErr := r.updateServiceStatus(csc, service.Name, crName, operatorv1alpha1.ServiceRunning)
	if stateUpdateErr != nil {
		klog.Error("Failed to update status")
		return stateUpdateErr
	}

	return nil
}

func (r *ReconcileOperandRequest) existingCustomResource(unstruct unstructured.Unstructured, service *operatorv1alpha1.ConfigService, namespace string, csc *operatorv1alpha1.OperandConfig) error {
	kind := unstruct.Object["kind"].(string)

	var found bool
	for crName, crdConfig := range service.Spec {
		// Compare the name of OperandConfig and CRD name
		if strings.EqualFold(kind, crName) {
			found = true
			klog.V(3).Info("Found OperandConfig spec for custom resource: " + kind)
			updateErr := r.updateCustomResource(unstruct, service, namespace, crName, csc, crdConfig.Raw)
			if updateErr != nil {
				klog.Error("Failed to create custom resource: ", updateErr)
				return updateErr
			}
		}
	}
	if !found {
		deleteErr := r.deleteCustomResource(unstruct, service, namespace, csc)
		if deleteErr != nil {
			klog.Error("Failed to delete custom resource: ", deleteErr)
			return deleteErr
		}
	}
	return nil
}

func (r *ReconcileOperandRequest) updateCustomResource(unstruct unstructured.Unstructured, service *operatorv1alpha1.ConfigService, namespace, crName string, csc *operatorv1alpha1.OperandConfig, crConfig []byte) error {

	kind := unstruct.Object["kind"].(string)
	apiversion := unstruct.Object["apiVersion"].(string)
	name := unstruct.Object["metadata"].(map[string]interface{})["name"].(string)

	// Update the CR
	existingCR := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiversion,
			"kind":       kind,
		},
	}

	crGetErr := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &existingCR)

	if crGetErr != nil {
		stateUpdateErr := r.updateServiceStatus(csc, service.Name, crName, operatorv1alpha1.ServiceFailed)
		if stateUpdateErr != nil {
			klog.Error("Failed to update status")
			return stateUpdateErr
		}
		klog.Errorf("Failed to get the Custom Resource %s: %s", crName, crGetErr)
		return crGetErr
	}

	if checkLabel(existingCR, map[string]string{constant.OpreqLabel: "true"}) {

		specJSONString, _ := json.Marshal(unstruct.Object["spec"])

		// Merge CR template spec and OperandConfig spec
		mergedCR := util.MergeCR(specJSONString, crConfig)
		CRgeneration := existingCR.Object["metadata"].(map[string]interface{})["generation"]
		existingCR.Object["spec"] = mergedCR
		if crUpdateErr := r.client.Update(context.TODO(), &existingCR); crUpdateErr != nil {
			stateUpdateErr := r.updateServiceStatus(csc, service.Name, crName, operatorv1alpha1.ServiceFailed)
			if stateUpdateErr != nil {
				klog.Error("Failed to update status")
				return stateUpdateErr
			}
			klog.Errorf("Failed to Update the Custom Resource %s: %s", crName, crUpdateErr)
			return crUpdateErr
		}
		UpdatedCR := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": apiversion,
				"kind":       kind,
			},
		}

		crGetErr = r.client.Get(context.TODO(), types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, &UpdatedCR)

		if crGetErr != nil {
			stateUpdateErr := r.updateServiceStatus(csc, service.Name, crName, operatorv1alpha1.ServiceFailed)
			if stateUpdateErr != nil {
				klog.Error("Failed to update status")
				return stateUpdateErr
			}
			klog.Errorf("Failed to get the Custom Resource %s: %s", crName, crGetErr)
			return crGetErr
		}
		if UpdatedCR.Object["metadata"].(map[string]interface{})["generation"] != CRgeneration {
			klog.V(2).Info("Finish updating the Custom Resource: ", crName)
		}
		stateUpdateErr := r.updateServiceStatus(csc, service.Name, crName, operatorv1alpha1.ServiceRunning)
		if stateUpdateErr != nil {
			klog.Error("Failed to update status")
			return stateUpdateErr
		}
	}

	return nil
}

func (r *ReconcileOperandRequest) deleteCustomResource(unstruct unstructured.Unstructured, service *operatorv1alpha1.ConfigService, namespace string, csc *operatorv1alpha1.OperandConfig) error {

	// Get the kind of CR
	kind := unstruct.Object["kind"].(string)
	apiversion := unstruct.Object["apiVersion"].(string)
	name := unstruct.Object["metadata"].(map[string]interface{})["name"].(string)

	crShouldBeDeleted := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiversion,
			"kind":       kind,
		},
	}
	getError := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &crShouldBeDeleted)
	if getError != nil && !errors.IsNotFound(getError) {
		klog.Error("Failed to get the custom resource should be deleted with name: ", name, getError)
		return getError
	}
	if errors.IsNotFound(getError) {
		klog.V(3).Infof("There is no custom resource: %s from custom resource definition: %s", name, kind)
	} else {
		if checkLabel(crShouldBeDeleted, map[string]string{constant.OpreqLabel: "true"}) && !checkLabel(crShouldBeDeleted, map[string]string{constant.NotUninstallLabel: "true"}) {
			klog.V(3).Infof("Deleting custom resource: %s from custom resource definition: %s", name, kind)
			deleteErr := r.client.Delete(context.TODO(), &crShouldBeDeleted)
			if deleteErr != nil {
				klog.Error("Failed to delete the custom resource should be deleted: ", deleteErr)
				return deleteErr
			}
			err := wait.PollImmediate(time.Second*20, time.Minute*10, func() (bool, error) {
				if strings.EqualFold(kind, "OperandRequest") {
					return true, nil
				}
				klog.V(3).Infof("Waiting for CR %s is removed ...", kind)
				err := r.client.Get(context.TODO(), types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				},
					&unstruct)
				if errors.IsNotFound(err) {
					return true, nil
				}
				if err != nil {
					klog.Error("Failed to get the custom resource: ", err)
					return false, err
				}
				return false, nil
			})
			if err != nil {
				klog.Error("Failed to delete the custom resource should be deleted: ", err)
				return err
			}
			stateDeleteErr := r.deleteServiceStatus(csc, service.Name, kind)
			if stateDeleteErr != nil {
				klog.Error("Failed to clean up the deleted service status in the operand config: ", stateDeleteErr)
				return stateDeleteErr
			}
			klog.V(2).Infof("Finish deleting custom resource: %s from custom resource definition: %s", name, kind)
		}
	}
	return nil
}

func checkLabel(unstruct unstructured.Unstructured, labels map[string]string) bool {
	for k, v := range labels {
		if !hasLabel(unstruct, k) {
			return false
		}
		if unstruct.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})[k] != v {
			return false
		}
	}
	return true
}

func hasLabel(unstruct unstructured.Unstructured, labelName string) bool {
	if unstruct.Object["metadata"].(map[string]interface{})["labels"] == nil {
		return false
	}
	if _, ok := unstruct.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})[labelName]; !ok {
		return false
	}
	return true
}

func ensureLabel(unstruct unstructured.Unstructured, labels map[string]string) bool {
	if unstruct.Object["metadata"].(map[string]interface{})["labels"] == nil {
		unstruct.Object["metadata"].(map[string]interface{})["labels"] = make(map[string]interface{})
	}
	for k, v := range labels {
		unstruct.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})[k] = v
	}
	return true
}
