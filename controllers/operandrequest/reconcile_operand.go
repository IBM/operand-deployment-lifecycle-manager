//
// Copyright 2021 IBM Corporation
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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	constant "github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	util "github.com/IBM/operand-deployment-lifecycle-manager/controllers/util"
)

func (r *Reconciler) reconcileOperand(requestKey types.NamespacedName) *util.MultiErr {
	klog.V(1).Infof("Reconciling Operands for OperandRequest: %s", requestKey)
	merr := &util.MultiErr{}
	requestInstance, err := r.FetchOperandRequest(requestKey)
	if err != nil {
		merr.Add(err)
		return merr
	}
	// Update request status
	defer func() {
		requestInstance.UpdateClusterPhase()
		err := r.updateOperandRequestStatus(requestInstance)
		if err != nil {
			klog.Errorf("failed to update the status for OperandRequest %s/%s : %v", requestInstance.Namespace, requestInstance.Name, err)
		}
	}()

	if err := r.checkCustomResource(requestInstance); err != nil {
		klog.Error(err)
		merr.Add(err)
		return merr
	}
	for _, req := range requestInstance.Spec.Requests {
		registryKey := requestInstance.GetRegistryKey(req)
		configInstance, err := r.FetchOperandConfig(registryKey)
		if err != nil {
			klog.Error("failed to get the OperandConfig instance: ", err)
			merr.Add(err)
			continue
		}
		registryInstance, err := r.FetchOperandRegistry(registryKey)
		if err != nil {
			klog.Error("failed to get the OperandRegistry instance: ", err)
			merr.Add(err)
			continue
		}
		for _, operand := range req.Operands {

			opdRegistry := registryInstance.GetOperator(operand.Name)
			if opdRegistry == nil {
				klog.Warningf("Cannot find %s in the OperandRegistry instance %s in the namespace %s ", operand.Name, req.Registry, req.RegistryNamespace)
				continue
			}

			operatorName := opdRegistry.Name

			klog.V(3).Info("Looking for csv for the operator: ", operatorName)

			// Looking for the CSV
			namespace := r.GetOperatorNamespace(opdRegistry.InstallMode, opdRegistry.Namespace)

			sub, err := r.FetchSubscription(operatorName, namespace, opdRegistry.PackageName)

			if errors.IsNotFound(err) {
				klog.V(2).Infof("There is no Subscription %s or %s in the namespace %s", operatorName, opdRegistry.PackageName, namespace)
				continue
			}

			if _, ok := sub.Labels[constant.OpreqLabel]; !ok {
				// Subscription existing and not managed by OperandRequest controller
				klog.Warningf("Subscription %s in the namespace %s isn't created by ODLM", sub.Name, sub.Namespace)
			}

			csv, err := r.FetchClusterServiceVersion(sub)

			// If can't get CSV, requeue the request
			if err != nil {
				klog.Errorf("failed to get the ClusterServiceVersion for the Subscription %s in the namespace %s: %s", operatorName, namespace, err)
				merr.Add(err)
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorFailed, "")
				continue
			}

			if csv == nil {
				klog.Warningf("ClusterServiceVersion for the Subscription %s in the namespace %s is not ready yet, retry", operatorName, namespace)
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorInstalling, "")
				continue
			}

			if csv.Status.Phase == olmv1alpha1.CSVPhaseFailed {
				klog.Errorf("the ClusterServiceVersion for Subscription %s is Failed", operatorName)
				merr.Add(fmt.Errorf("the ClusterServiceVersion for Subscription %s is Failed", operatorName))
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorFailed, "")
				continue
			}
			if csv.Status.Phase != olmv1alpha1.CSVPhaseSucceeded {
				klog.Errorf("the ClusterServiceVersion for Subscription %s is not Ready", operatorName)
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorInstalling, "")
				continue
			}

			klog.V(3).Info("Generating customresource base on ClusterServiceVersion: ", csv.ObjectMeta.Name)
			requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorRunning, "")

			// Merge and Generate CR
			if operand.Kind == "" {
				// Check the requested Service Config if exist in specific OperandConfig
				opdConfig := configInstance.GetService(operand.Name)
				if opdConfig == nil {
					klog.V(2).Infof("There is no service: %s from the OperandConfig instance: %s/%s, Skip creating CR for it", operand.Name, req.RegistryNamespace, req.Registry)
					continue
				}
				err = r.reconcileCRwithConfig(opdConfig, opdRegistry.Namespace, csv)
			} else {
				err = r.reconcileCRwithRequest(requestInstance, operand, requestKey, csv)
			}

			if err != nil {
				klog.Error("failed to get create or update customresource: ", err)
				merr.Add(err)
				requestInstance.SetMemberStatus(operand.Name, "", operatorv1alpha1.ServiceFailed)
			}
			requestInstance.SetMemberStatus(operand.Name, "", operatorv1alpha1.ServiceRunning)
		}
	}
	if len(merr.Errors) != 0 {
		return merr
	}
	klog.V(1).Infof("Finished reconciling Operands for OperandRequest: %s", requestKey)
	return &util.MultiErr{}
}

// reconcileCRwithConfig merge and create custom resource base on OperandConfig and CSV alm-examples
func (r *Reconciler) reconcileCRwithConfig(service *operatorv1alpha1.ConfigService, namespace string, csv *olmv1alpha1.ClusterServiceVersion) error {
	almExamples := csv.ObjectMeta.Annotations["alm-examples"]

	// Create a slice for crTemplates
	var crTemplates []interface{}

	// Convert CR template string to slice
	crTemplatesErr := json.Unmarshal([]byte(almExamples), &crTemplates)
	if crTemplatesErr != nil {
		klog.Errorf("failed to convert alm-examples in the Subscription %s to slice: %s", service.Name, crTemplatesErr)
		return crTemplatesErr
	}

	merr := &util.MultiErr{}

	// Merge OperandConfig and ClusterServiceVersion alm-examples
	for _, crTemplate := range crTemplates {

		// Create an unstruct object for CR and request its value to CR template
		var unstruct unstructured.Unstructured
		unstruct.Object = crTemplate.(map[string]interface{})

		name := unstruct.Object["metadata"].(map[string]interface{})["name"].(string)

		getError := r.Reader.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, &unstruct)

		if getError != nil && !errors.IsNotFound(getError) {
			klog.Error("failed to get the custom resource should be deleted with name: ", name, getError)
			merr.Add(getError)
			continue
		} else if errors.IsNotFound(getError) {
			// Create Custom resource
			if createErr := r.compareConfigandExample(unstruct, service, namespace); createErr != nil {
				merr.Add(createErr)
				continue
			}
		} else {
			if checkLabel(unstruct, map[string]string{constant.OpreqLabel: "true"}) {
				// Update or Delete Custom resource
				if updateDeleteErr := r.existingCustomResource(unstruct, service, namespace); updateDeleteErr != nil {
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

// reconcileCRwithRequest merge and create custom resource base on OperandRequest and CSV alm-examples
func (r *Reconciler) reconcileCRwithRequest(requestInstance *operatorv1alpha1.OperandRequest, operand operatorv1alpha1.Operand, requestKey types.NamespacedName, csv *olmv1alpha1.ClusterServiceVersion) error {
	almExamples := csv.ObjectMeta.Annotations["alm-examples"]

	// Create a slice for crTemplates
	var crTemplates []interface{}

	// Convert CR template string to slice
	crTemplatesErr := json.Unmarshal([]byte(almExamples), &crTemplates)
	if crTemplatesErr != nil {
		klog.Errorf("failed to convert alm-examples in the Subscription %s to slice: %s", operand.Name, crTemplatesErr)
		return crTemplatesErr
	}

	merr := &util.MultiErr{}

	// Merge OperandConfig and ClusterServiceVersion alm-examples
	var found bool
	for _, crTemplate := range crTemplates {

		// Create an unstruct object for CR and request its value to CR template
		var unstruct unstructured.Unstructured

		unstruct.Object = crTemplate.(map[string]interface{})

		if unstruct.Object["kind"].(string) != operand.Kind {
			continue
		}

		found = true
		var name string
		if operand.InstanceName == "" {
			name = requestKey.Name
		} else {
			name = operand.InstanceName
		}

		unstruct.Object["metadata"].(map[string]interface{})["name"] = name
		unstruct.Object["metadata"].(map[string]interface{})["namespace"] = requestKey.Namespace

		err := r.Reader.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: requestKey.Namespace,
		}, &unstruct)

		if err != nil && !errors.IsNotFound(err) {
			klog.Error("failed to get the custom resource should be deleted with name: ", name, err)
			merr.Add(err)
			continue
		} else if errors.IsNotFound(err) {
			// Create Custom resource
			if err := r.createCustomResource(unstruct, requestKey.Namespace, operand.Kind, operand.Spec.Raw); err != nil {
				merr.Add(err)
				continue
			}
			requestInstance.SetMemberCRStatus(operand.Name, name, operand.Kind, unstruct.Object["apiVersion"].(string))
		} else {
			if checkLabel(unstruct, map[string]string{constant.OpreqLabel: "true"}) {
				// Update or Delete Custom resource
				klog.V(3).Info("Found OperandConfig spec for custom resource: " + operand.Kind)
				if err := r.updateCustomResource(unstruct, requestKey.Namespace, operand.Kind, operand.Spec.Raw); err != nil {
					klog.Error("failed to create custom resource: ", err)
					return err
				}
			} else {
				klog.V(2).Info("Skip the custom resource not created by ODLM")
			}
		}
	}
	if !found {
		klog.Errorf("not found CRD with Kind %s in the alm-example", operand.Kind)
	}
	if len(merr.Errors) != 0 {
		return merr
	}
	return nil
}

// deleteAllCustomResource remove custom resource base on OperandConfig and CSV alm-examples
func (r *Reconciler) deleteAllCustomResource(csv *olmv1alpha1.ClusterServiceVersion, requestInstance *operatorv1alpha1.OperandRequest, csc *operatorv1alpha1.OperandConfig, operandName, namespace string) error {

	customeResourceMap := make(map[string]operatorv1alpha1.OperandCRMember)
	for _, member := range requestInstance.Status.Members {
		if len(member.OperandCRList) != 0 {
			for _, cr := range member.OperandCRList {
				customeResourceMap[member.Name+"/"+cr.Kind+"/"+cr.Name] = cr
			}
		}
	}

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
		if err := r.deleteCustomResource(crShouldBeDeleted, requestInstance.Namespace); err != nil {
			merr.Add(err)
		}
		operatorName := strings.Split(index, "/")[0]
		requestInstance.RemoveMemberCRStatus(operatorName, opdMember.Name, opdMember.Kind)
	}
	if len(merr.Errors) != 0 {
		return merr
	}

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
		klog.Errorf("failed to convert alm-examples in the Subscription %s to slice: %s", service.Name, crTemplatesErr)
		return crTemplatesErr
	}

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
				getError := r.Reader.Get(ctx, types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				}, &unstruct)
				if getError != nil && !errors.IsNotFound(getError) {
					klog.Errorf("failed to get the custom resource %s in the namespace %s: %s", name, namespace, getError)
					merr.Add(getError)
					continue
				}
				if errors.IsNotFound(getError) {
					klog.V(2).Info("Finish Deleting the CR: " + kind)
					continue
				}
				if checkLabel(unstruct, map[string]string{constant.OpreqLabel: "true"}) {
					deleteErr := r.deleteCustomResource(unstruct, namespace)
					if deleteErr != nil {
						klog.Errorf("failed to delete custom resource %s in the namespace %s: %s", name, namespace, deleteErr)
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

func (r *Reconciler) compareConfigandExample(unstruct unstructured.Unstructured, service *operatorv1alpha1.ConfigService, namespace string) error {
	kind := unstruct.Object["kind"].(string)

	for crdName, crdConfig := range service.Spec {
		// Compare the name of OperandConfig and CRD name
		if strings.EqualFold(kind, crdName) {
			klog.V(3).Info("Found OperandConfig spec for custom resource: " + kind)
			createErr := r.createCustomResource(unstruct, namespace, crdName, crdConfig.Raw)
			if createErr != nil {
				klog.Error("failed to create custom resource: ", createErr)
				return createErr
			}
		}
	}
	return nil
}

func (r *Reconciler) createCustomResource(unstruct unstructured.Unstructured, namespace, crName string, crConfig []byte) error {

	//Convert CR template spec to string
	specJSONString, _ := json.Marshal(unstruct.Object["spec"])

	// Merge CR template spec and OperandConfig spec
	mergedCR := util.MergeCR(specJSONString, crConfig)

	unstruct.Object["spec"] = mergedCR
	unstruct.Object["metadata"].(map[string]interface{})["namespace"] = namespace

	ensureLabel(unstruct, map[string]string{constant.OpreqLabel: "true"})

	// Creat the CR
	crCreateErr := r.Create(ctx, &unstruct)
	if crCreateErr != nil && !errors.IsAlreadyExists(crCreateErr) {
		klog.Errorf("failed to create the Custom Resource %s: %s ", crName, crCreateErr)
		return crCreateErr
	}

	klog.V(2).Info("Finish creating the Custom Resource: ", crName)

	return nil
}

func (r *Reconciler) existingCustomResource(unstruct unstructured.Unstructured, service *operatorv1alpha1.ConfigService, namespace string) error {
	kind := unstruct.Object["kind"].(string)

	var found bool
	for crName, crdConfig := range service.Spec {
		// Compare the name of OperandConfig and CRD name
		if strings.EqualFold(kind, crName) {
			found = true
			klog.V(3).Info("Found OperandConfig spec for custom resource: " + kind)
			updateErr := r.updateCustomResource(unstruct, namespace, crName, crdConfig.Raw)
			if updateErr != nil {
				klog.Error("failed to create custom resource: ", updateErr)
				return updateErr
			}
		}
	}
	if !found {
		deleteErr := r.deleteCustomResource(unstruct, namespace)
		if deleteErr != nil {
			klog.Error("failed to delete custom resource: ", deleteErr)
			return deleteErr
		}
	}
	return nil
}

func (r *Reconciler) updateCustomResource(unstruct unstructured.Unstructured, namespace, crName string, crConfig []byte) error {

	kind := unstruct.Object["kind"].(string)
	apiversion := unstruct.Object["apiVersion"].(string)
	name := unstruct.Object["metadata"].(map[string]interface{})["name"].(string)

	// Update the CR
	err := wait.PollImmediate(time.Millisecond*250, time.Second*5, func() (bool, error) {

		existingCR := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": apiversion,
				"kind":       kind,
			},
		}

		crGetErr := r.Reader.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, &existingCR)

		if crGetErr != nil {
			klog.Errorf("failed to get the Custom Resource %s: %s", crName, crGetErr)
			return false, crGetErr
		}

		if checkLabel(existingCR, map[string]string{constant.OpreqLabel: "true"}) {

			specJSONString, _ := json.Marshal(unstruct.Object["spec"])

			// Merge CR template spec and OperandConfig spec
			mergedCR := util.MergeCR(specJSONString, crConfig)

			CRgeneration := existingCR.Object["metadata"].(map[string]interface{})["generation"]

			if reflect.DeepEqual(existingCR.Object["spec"], mergedCR) {
				return true, nil
			}

			klog.V(2).Infof("updating custom resource with apiversion: %s, kind: %s, %s/%s", apiversion, kind, namespace, name)

			existingCR.Object["spec"] = mergedCR
			crUpdateErr := r.Update(ctx, &existingCR)

			if crUpdateErr != nil {
				klog.Errorf("failed to update the Custom Resource %s: %s", crName, crUpdateErr)
				return false, crUpdateErr
			}

			UpdatedCR := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": apiversion,
					"kind":       kind,
				},
			}

			err := r.Reader.Get(ctx, types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			}, &UpdatedCR)

			if err != nil {
				klog.Errorf("failed to get the Custom Resource %s: %s", crName, err)
				return false, err
			}

			if UpdatedCR.Object["metadata"].(map[string]interface{})["generation"] != CRgeneration {
				klog.V(2).Info("Finish updating the Custom Resource: ", crName)
			}
		}
		return true, nil
	})

	if err != nil {
		klog.Error("failed to update the custom resource: ", err)
		return err
	}

	return nil
}

func (r *Reconciler) deleteCustomResource(unstruct unstructured.Unstructured, namespace string) error {

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
	getError := r.Reader.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &crShouldBeDeleted)
	if getError != nil && !errors.IsNotFound(getError) {
		klog.Error("failed to get the custom resource should be deleted with name: ", name, getError)
		return getError
	}
	if errors.IsNotFound(getError) {
		klog.V(3).Infof("There is no custom resource: %s from custom resource definition: %s", name, kind)
	} else {
		if checkLabel(crShouldBeDeleted, map[string]string{constant.OpreqLabel: "true"}) && !checkLabel(crShouldBeDeleted, map[string]string{constant.NotUninstallLabel: "true"}) {
			klog.V(3).Infof("Deleting custom resource: %s from custom resource definition: %s", name, kind)
			deleteErr := r.Delete(ctx, &crShouldBeDeleted)
			if deleteErr != nil {
				klog.Error("failed to delete the custom resource should be deleted: ", deleteErr)
				return deleteErr
			}
			err := wait.PollImmediate(time.Second*20, time.Minute*10, func() (bool, error) {
				if strings.EqualFold(kind, "OperandRequest") {
					return true, nil
				}
				klog.V(3).Infof("Waiting for CR %s is removed ...", kind)
				err := r.Reader.Get(ctx, types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				},
					&unstruct)
				if errors.IsNotFound(err) {
					return true, nil
				}
				if err != nil {
					klog.Error("failed to get the custom resource: ", err)
					return false, err
				}
				return false, nil
			})
			if err != nil {
				klog.Error("failed to delete the custom resource should be deleted: ", err)
				return err
			}
			klog.V(2).Infof("Finish deleting custom resource: %s from custom resource definition: %s", name, kind)
		}
	}
	return nil
}

func (r *Reconciler) checkCustomResource(requestInstance *operatorv1alpha1.OperandRequest) error {
	klog.V(3).Infof("checking the custom resource should be deleted from OperandRequest %s/%s", requestInstance.Namespace, requestInstance.Name)

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
		if err := r.deleteCustomResource(crShouldBeDeleted, requestInstance.Namespace); err != nil {
			merr.Add(err)
		}
		operatorName := strings.Split(index, "/")[0]
		requestInstance.RemoveMemberCRStatus(operatorName, opdMember.Name, opdMember.Kind)
	}

	if len(merr.Errors) != 0 {
		return merr
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
