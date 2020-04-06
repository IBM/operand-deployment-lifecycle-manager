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
	util "github.com/IBM/operand-deployment-lifecycle-manager/pkg/util"
)

const t = "true"

func (r *ReconcileOperandRequest) reconcileOperand(requestInstance *operatorv1alpha1.OperandRequest) *util.MultiErr {
	klog.V(1).Info("Reconciling Operand")
	merr := &util.MultiErr{}

	for _, req := range requestInstance.Spec.Requests {
		for _, operand := range req.Operands {
			configInstance, err := r.getConfigInstance(req.Registry, req.RegistryNamespace)
			if err != nil {
				klog.Error("Failed to get Operand Config Instance: ", err)
				merr.Add(err)
				continue
			}
			// Check the requested Service Config if exist in specific OperandConfig
			svc := r.getServiceFromConfigInstance(operand.Name, configInstance)
			if svc != nil {
				klog.V(3).Info("Reconciling custom resource: ", svc.Name)
				// Looking for the CSV
				csv, err := r.getClusterServiceVersion(svc.Name)

				// If can't get CSV, requeue the request
				if err != nil {
					klog.Error("Failed to get Cluster Service Version: ", err)
					merr.Add(err)
					continue
				}

				if csv == nil {
					continue
				}

				klog.V(3).Info("Generating custom resource base on Cluster Service Version: ", csv.ObjectMeta.Name)

				// Merge and Generate CR
				err = r.reconcileCr(svc, csv, configInstance)
				if err != nil {
					klog.Error("Error when get create or update custom resource: ", err)
					merr.Add(err)
				}
			}
		}
	}
	if len(merr.Errors) != 0 {
		return merr
	}
	return &util.MultiErr{}
}

// getCSV retrieves the Cluster Service Version
func (r *ReconcileOperandRequest) getClusterServiceVersion(subName string) (*olmv1alpha1.ClusterServiceVersion, error) {
	klog.V(3).Info("Looking for the Cluster Service Version ", "in Subscription: ", subName)
	subs, listSubErr := r.olmClient.OperatorsV1alpha1().Subscriptions("").List(metav1.ListOptions{
		LabelSelector: "operator.ibm.com/opreq-control",
	})
	if listSubErr != nil {
		klog.Error("Fail to list subscriptions: ", listSubErr)
		return nil, listSubErr
	}
	var csvName, csvNamespace string
	for _, s := range subs.Items {
		if s.Name == subName {
			if s.Status.CurrentCSV == "" {
				klog.V(3).Info("There is no Cluster Service Version for the Subscription: ", subName)
				return nil, nil
			}
			csvName = s.Status.CurrentCSV
			csvNamespace = s.Namespace
			csv, getCSVErr := r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(csvNamespace).Get(csvName, metav1.GetOptions{})
			if getCSVErr != nil {
				if errors.IsNotFound(getCSVErr) {
					continue
				}
				klog.Error("Fail to get Cluster Service Version: ", getCSVErr)
				return nil, getCSVErr
			}
			klog.V(3).Info("Get Cluster Service Version: ", csvName, " in namespace: ", csvNamespace)
			return csv, nil
		}
	}
	klog.V(3).Info("There is no Cluster Service Version for: ", subName)
	return nil, nil
}

// reconcileCr merge and create custome resource base on OperandConfig and CSV alm-examples
func (r *ReconcileOperandRequest) reconcileCr(service *operatorv1alpha1.ConfigService, csv *olmv1alpha1.ClusterServiceVersion, csc *operatorv1alpha1.OperandConfig) error {
	almExamples := csv.ObjectMeta.Annotations["alm-examples"]
	namespace := csv.ObjectMeta.Namespace

	// Create a slice for crTemplates
	var crTemplates []interface{}

	// Convert CR template string to slice
	crTemplatesErr := json.Unmarshal([]byte(almExamples), &crTemplates)
	if crTemplatesErr != nil {
		klog.Error("Fail to convert alm-examples to slice: ", crTemplatesErr, " Subscription: ", service.Name)
		return crTemplatesErr
	}

	merr := &util.MultiErr{}

	// Merge OperandConfig and Cluster Service Version alm-examples
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
			klog.Error("Failed to get the custom resource should be deleted: ", getError)
			merr.Add(getError)
			continue
		} else if errors.IsNotFound(getError) {
			// Create Custom resource
			if createErr := r.compareConfigandExample(unstruct, service, namespace, csc); createErr != nil {
				merr.Add(createErr)
				continue
			}
		} else {
			if unstruct.Object["metadata"].(map[string]interface{})["labels"] != nil && unstruct.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["operator.ibm.com/opreq-control"] == t {
				// Update or Delete Custom resource
				if updateDeleteErr := r.existingCustomResource(unstruct, service, namespace, csc); updateDeleteErr != nil {
					merr.Add(updateDeleteErr)
					continue
				}
			} else {
				klog.V(2).Info("Skip the custom resource created by other users")
			}
		}
	}
	if len(merr.Errors) != 0 {
		return merr
	}
	return nil
}

// deleteAllCustomResource remove custome resource base on OperandConfig and CSV alm-examples
func (r *ReconcileOperandRequest) deleteAllCustomResource(csv *olmv1alpha1.ClusterServiceVersion, csc *operatorv1alpha1.OperandConfig, operandName string) error {

	service := r.getServiceFromConfigInstance(operandName, csc)
	if service == nil {
		return nil
	}
	almExamples := csv.ObjectMeta.Annotations["alm-examples"]
	klog.V(2).Info("Delete all the custom resource from Subscription ", service.Name)
	namespace := csv.ObjectMeta.Namespace

	// Create a slice for crTemplates
	var crTemplates []interface{}

	// Convert CR template string to slice
	crTemplatesErr := json.Unmarshal([]byte(almExamples), &crTemplates)
	if crTemplatesErr != nil {
		klog.Error("Fail to convert alm-examples to slice: ", crTemplatesErr)
		return crTemplatesErr
	}

	merr := &util.MultiErr{}

	// Merge OperandConfig and Cluster Service Version alm-examples
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
					klog.Error("Failed to get the custom resource should be deleted: ", getError)
					merr.Add(getError)
					continue
				}
				if errors.IsNotFound(getError) {
					klog.V(2).Info("Finish Deleting the CR: " + kind)
					stateDeleteErr := r.deleteServiceStatus(csc, service.Name, crdName)
					if stateDeleteErr != nil {
						klog.Error("Failed to clean up the deleted service status in the operand config: ", stateDeleteErr)
						merr.Add(stateDeleteErr)
					}
					continue
				}
				if unstruct.Object["metadata"].(map[string]interface{})["labels"] != nil && unstruct.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["operator.ibm.com/opreq-control"] == t {
					deleteErr := r.deleteCustomResource(unstruct, service, namespace, csc)
					if deleteErr != nil {
						klog.Error("Failed to delete custom resource: ", deleteErr)
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

// Get the OperandConfig instance with the name and namespace
func (r *ReconcileOperandRequest) getConfigInstance(name, namespace string) (*operatorv1alpha1.OperandConfig, error) {
	klog.V(3).Info("Get the OperandConfig instance from the name: ", name, " namespace: ", namespace)
	config := &operatorv1alpha1.OperandConfig{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, config); err != nil {
		return nil, err
	}
	return config, nil
}

func (r *ReconcileOperandRequest) getServiceFromConfigInstance(operandName string, configInstance *operatorv1alpha1.OperandConfig) *operatorv1alpha1.ConfigService {
	klog.V(3).Info("Get ConfigService from the OperandConfig instance: ", configInstance.ObjectMeta.Name, " and operand name: ", operandName)
	for _, s := range configInstance.Spec.Services {
		if s.Name == operandName {
			return &s
		}
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
	if unstruct.Object["metadata"].(map[string]interface{})["labels"] == nil {
		klog.V(3).Info("Adding ODLM label in the custom resource")
		unstruct.Object["metadata"].(map[string]interface{})["labels"] = make(map[string]interface{})
	}
	unstruct.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["operator.ibm.com/opreq-control"] = t
	// Creat or Update the CR
	crCreateErr := r.client.Create(context.TODO(), &unstruct)
	if crCreateErr != nil && !errors.IsAlreadyExists(crCreateErr) {
		stateUpdateErr := r.updateServiceStatus(csc, service.Name, crName, operatorv1alpha1.ServiceFailed)
		if stateUpdateErr != nil {
			klog.Error("Fail to update status")
			return stateUpdateErr
		}
		klog.Error("Fail to Create the Custom Resource: ", crName, ". Error message: ", crCreateErr)
		return crCreateErr
	}

	klog.V(2).Info("Finish creating the Custom Resource: ", crName)
	stateUpdateErr := r.updateServiceStatus(csc, service.Name, crName, operatorv1alpha1.ServiceRunning)
	if stateUpdateErr != nil {
		klog.Error("Fail to update status")
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
			klog.Error("Fail to update status")
			return stateUpdateErr
		}
		klog.Error("Fail to Get the Custom Resource: ", crName, ". Error message: ", crGetErr)
		return crGetErr
	}

	if existingCR.Object["metadata"].(map[string]interface{})["labels"] != nil && existingCR.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["operator.ibm.com/opreq-control"] == t {

		specJSONString, _ := json.Marshal(unstruct.Object["spec"])

		// Merge CR template spec and OperandConfig spec
		mergedCR := util.MergeCR(specJSONString, crConfig)

		existingCR.Object["spec"] = mergedCR
		if crUpdateErr := r.client.Update(context.TODO(), &existingCR); crUpdateErr != nil {
			stateUpdateErr := r.updateServiceStatus(csc, service.Name, crName, operatorv1alpha1.ServiceFailed)
			if stateUpdateErr != nil {
				klog.Error("Fail to update status")
				return stateUpdateErr
			}
			klog.Error("Fail to Update the Custom Resource ", crName, ". Error message: ", crUpdateErr)
			return crUpdateErr
		}
		klog.V(2).Info("Finish updating the Custom Resource: ", crName)
		stateUpdateErr := r.updateServiceStatus(csc, service.Name, crName, operatorv1alpha1.ServiceRunning)
		if stateUpdateErr != nil {
			klog.Error("Fail to update status")
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

	crShouldBeDeleted := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiversion,
			"kind":       kind,
		},
	}
	getError := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, crShouldBeDeleted)
	if getError != nil && !errors.IsNotFound(getError) {
		klog.Error("Failed to get the custom resource should be deleted: ", getError)
		return getError
	}
	if errors.IsNotFound(getError) {
		klog.V(3).Infof("There is no custom resource: %s from custom resource definition: %s", name, kind)
	} else {
		if crShouldBeDeleted.Object["metadata"].(map[string]interface{})["labels"] != nil && crShouldBeDeleted.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["operator.ibm.com/opreq-control"] == t {
			klog.V(3).Infof("Deleting custom resource: %s from custom resource definition: %s", name, kind)
			deleteErr := r.client.Delete(context.TODO(), crShouldBeDeleted)
			if deleteErr != nil {
				klog.Error("Failed to delete the custom resource should be deleted: ", deleteErr)
				return deleteErr
			}
			klog.V(3).Info("Waiting for CR: " + kind + " is deleted")
			err := wait.PollImmediate(time.Second*20, time.Minute*10, func() (bool, error) {
				klog.V(3).Info("Checking for CR: " + kind + " is deleted")
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
