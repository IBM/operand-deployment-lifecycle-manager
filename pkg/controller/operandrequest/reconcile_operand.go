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

func (r *ReconcileOperandRequest) reconcileOperand(requestInstance *operatorv1alpha1.OperandRequest) *multiErr {
	klog.V(1).Info("Reconciling Operand")
	merr := &multiErr{}

	for _, req := range requestInstance.Spec.Requests {
		for _, operand := range req.Operands {
			configInstance, err := r.getConfigInstance(req.Registry, req.RegistryNamespace)
			if err != nil {
				klog.Error(err)
				merr.Add(err)
				continue
			}
			// Check the requested Service Config if exist in specific OperandConfig
			svc := r.getServiceFromConfigInstance(operand.Name, configInstance)
			if svc != nil {
				klog.V(4).Info("Reconciling custom resource: ", svc.Name)
				// Looking for the CSV
				csv, err := r.getClusterServiceVersion(svc.Name)

				// If can't get CSV, requeue the request
				if err != nil {
					klog.Error(err)
					merr.Add(err)
					continue
				}

				if csv == nil {
					continue
				}

				klog.V(4).Info("Generating custom resource base on Cluster Service Version: ", csv.ObjectMeta.Name)

				// Merge and Generate CR
				err = r.createUpdateCr(svc, csv, configInstance)
				if err != nil {
					merr.Add(err)
				}
			}
		}
	}
	if len(merr.errors) != 0 {
		return merr
	}
	return &multiErr{}
}

// getCSV retrieves the Cluster Service Version
func (r *ReconcileOperandRequest) getClusterServiceVersion(subName string) (*olmv1alpha1.ClusterServiceVersion, error) {
	klog.V(2).Info("Looking for the Cluster Service Version ", "in Subscription: ", subName)
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
				klog.V(4).Info("There is no Cluster Service Version for the Subscription: ", subName)
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
			klog.V(2).Info("Get Cluster Service Version: ", csvName, " in namespace: ", csvNamespace)
			return csv, nil
		}
	}
	klog.V(2).Info("There is no Cluster Service Version for: ", subName)
	return nil, nil
}

// createUpdateCr merge and create custome resource base on OperandConfig and CSV alm-examples
func (r *ReconcileOperandRequest) createUpdateCr(service *operatorv1alpha1.ConfigService, csv *olmv1alpha1.ClusterServiceVersion, csc *operatorv1alpha1.OperandConfig) error {
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

	merr := &multiErr{}

	// Merge OperandConfig and Cluster Service Version alm-examples
	for _, crTemplate := range crTemplates {

		// Create an unstruct object for CR and request its value to CR template
		var unstruct unstructured.Unstructured
		unstruct.Object = crTemplate.(map[string]interface{})

		// Get the kind of CR
		kind := unstruct.Object["kind"].(string)
		apiversion := unstruct.Object["apiVersion"].(string)
		name := unstruct.Object["metadata"].(map[string]interface{})["name"].(string)

		var found bool
		for crdName, crConfig := range service.Spec {
			// Compare the name of OperandConfig and CRD name
			if strings.EqualFold(kind, crdName) {
				klog.V(4).Info("Found OperandConfig spec for custom resource: " + kind)
				found = true
				//Convert CR template spec to string
				specJSONString, _ := json.Marshal(unstruct.Object["spec"])

				// Merge CR template spec and OperandConfig spec
				mergedCR := util.MergeCR(specJSONString, crConfig.Raw)

				unstruct.Object["spec"] = mergedCR
				unstruct.Object["metadata"].(map[string]interface{})["namespace"] = namespace
				if (unstruct.Object["metadata"].(map[string]interface{})["labels"] == nil) {
					klog.V(4).Info("Adding ODLM label in the custom resource")
					unstruct.Object["metadata"].(map[string]interface{})["labels"] = make(map[string]interface{})
				}
				unstruct.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["operator.ibm.com/opreq-control"] = "true"

				// Creat or Update the CR
				crCreateErr := r.client.Create(context.TODO(), &unstruct)
				if crCreateErr != nil && !errors.IsAlreadyExists(crCreateErr) {
					stateUpdateErr := r.updateServiceStatus(csc, service.Name, crdName, operatorv1alpha1.ServiceFailed)
					if stateUpdateErr != nil {
						merr.Add(stateUpdateErr)
					}
					klog.Error("Fail to Create the Custom Resource: ", crdName, ". Error message: ", crCreateErr)
					merr.Add(crCreateErr)

				} else if errors.IsAlreadyExists(crCreateErr) {
					existingCR := &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": apiversion,
							"kind":       kind,
						},
					}

					crGetErr := r.client.Get(context.TODO(), types.NamespacedName{
						Name:      name,
						Namespace: namespace,
					}, existingCR)

					if crGetErr != nil {
						stateUpdateErr := r.updateServiceStatus(csc, service.Name, crdName, operatorv1alpha1.ServiceFailed)
						if stateUpdateErr != nil {
							merr.Add(stateUpdateErr)
						}
						klog.Error("Fail to Get the Custom Resource: ", crdName, ". Error message: ", crGetErr)
						merr.Add(crGetErr)
						continue
					}
					existingCR.Object["spec"] = unstruct.Object["spec"]
					if crUpdateErr := r.client.Update(context.TODO(), existingCR); crUpdateErr != nil {
						stateUpdateErr := r.updateServiceStatus(csc, service.Name, crdName, operatorv1alpha1.ServiceFailed)
						if stateUpdateErr != nil {
							merr.Add(stateUpdateErr)
						}
						klog.Error("Fail to Update the Custom Resource ", crdName, ". Error message: ", crUpdateErr)
						merr.Add(crUpdateErr)
						continue
					}
					klog.V(2).Info("Finish updating the Custom Resource: ", crdName)
					stateUpdateErr := r.updateServiceStatus(csc, service.Name, crdName, operatorv1alpha1.ServiceRunning)
					if stateUpdateErr != nil {
						merr.Add(stateUpdateErr)
					}
				} else {
					klog.V(2).Info("Finish creating the Custom Resource: ", crdName)
					stateUpdateErr := r.updateServiceStatus(csc, service.Name, crdName, operatorv1alpha1.ServiceRunning)
					if stateUpdateErr != nil {
						merr.Add(stateUpdateErr)
					}
				}
			}
		}
		if (!found) {
			crShouldBeDeleted := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": apiversion,
					"kind":       kind,
				},
			}
			getError := r.client.Get(context.TODO(),types.NamespacedName{
				Name: name,
				Namespace: namespace,
			}, crShouldBeDeleted)
			if getError != nil && !errors.IsNotFound(getError) {
				klog.Error(getError)
				merr.Add(getError)
				continue
			}
			if !errors.IsNotFound(getError) && crShouldBeDeleted.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["operator.ibm.com/opreq-control"] == "true"  {
				klog.V(3).Infof("Deleting custom resource: %s from custom resource definition: %s", name, kind)
				deleteErr := r.client.Delete(context.TODO(),crShouldBeDeleted)
				if deleteErr != nil {
					klog.Error(deleteErr)
					merr.Add(deleteErr)
					continue
				}
				stateDeleteErr := r.deleteServiceStatus(csc, service.Name, util.Lcfirst(kind))
				if stateDeleteErr != nil {
					klog.Error("Failed to clean up the deleted service status in the operand config: ", stateDeleteErr)
					merr.Add(stateDeleteErr)
					continue
				}
				klog.V(3).Infof("Finish deleting custom resource: %s from custom resource definition: %s", name, kind)			}
		}
	}
	if len(merr.errors) != 0 {
		return merr
	}
	return nil
}

// deleteCr remove custome resource base on OperandConfig and CSV alm-examples
func (r *ReconcileOperandRequest) deleteCr(csv *olmv1alpha1.ClusterServiceVersion, csc *operatorv1alpha1.OperandConfig, operandName string) error {

	service := r.getServiceFromConfigInstance(operandName, csc)

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

	merr := &multiErr{}

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
				getError := r.client.Get(context.TODO(),types.NamespacedName{
					Name: name,
					Namespace: namespace,
				},  &unstruct)
				if getError != nil && !errors.IsNotFound(getError) {
					klog.Error(getError)
					merr.Add(getError)
					continue
				}
				if errors.IsNotFound(getError) {
					klog.V(4).Info("Deleted the CR: " + kind)
					stateDeleteErr := r.deleteServiceStatus(csc, service.Name, crdName)
					if stateDeleteErr != nil {
						klog.Error("Failed to clean up the deleted service status in the operand config: ", stateDeleteErr)
						merr.Add(stateDeleteErr)
					}
					continue
				}
				if unstruct.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["operator.ibm.com/opreq-control"] == "true" {
					crDeleteErr := r.client.Delete(context.TODO(), &unstruct)
					if crDeleteErr != nil && !errors.IsNotFound(crDeleteErr) {
						klog.Error("Failed to delete the custom resource: ", crDeleteErr)
						merr.Add(crDeleteErr)
						continue
					}
					klog.V(4).Info("Waiting for CR: " + kind + " is deleted")
					err := wait.PollImmediate(time.Second*20, time.Minute*10, func() (bool, error) {
						klog.V(4).Info("Checking for CR: " + kind + " is deleted")
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
						klog.Error(err)
						merr.Add(err)
						continue
					}
					stateDeleteErr := r.deleteServiceStatus(csc, service.Name, crdName)
					if stateDeleteErr != nil {
						klog.Error("Failed to clean up the deleted service status in the operand config: ", stateDeleteErr)
						merr.Add(stateDeleteErr)
						continue
					}
					klog.V(4).Info("Deleted the CR: " + kind)
				}
			}

		}
	}
	if len(merr.errors) != 0 {
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
	klog.V(4).Info("Get ConfigService from the OperandConfig instance: ", configInstance.ObjectMeta.Name, " and operand name: ", operandName)
	for _, s := range configInstance.Spec.Services {
		if s.Name == operandName {
			return &s
		}
	}
	return nil
}
