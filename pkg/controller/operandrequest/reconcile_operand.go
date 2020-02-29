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

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	util "github.com/IBM/operand-deployment-lifecycle-manager/pkg/util"
)

func (r *ReconcileOperandRequest) reconcileOperand(serviceConfigs map[string]operatorv1alpha1.ConfigService, csc *operatorv1alpha1.OperandConfig) *multiErr {
	reqLogger := log.WithValues()
	reqLogger.Info("Reconciling Operand")
	merr := &multiErr{}
	for _, service := range serviceConfigs {
		if service.State == Present {
			reqLogger.Info(fmt.Sprintf("Reconciling custom resource %s", service.Name))
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

			reqLogger.Info(fmt.Sprintf("Generating custom resource base on Cluster Service Version %s", csv.ObjectMeta.Name))

			// Merge and Generate CR
			err = r.createUpdateCr(service, csv, csc)
			if err != nil {
				merr.Add(err)
			}
		}
	}
	if len(merr.errors) != 0 {
		return merr
	}
	return &multiErr{}
}

func (r *ReconcileOperandRequest) fetchConfigs(csc *operatorv1alpha1.OperandConfig, cr *operatorv1alpha1.OperandRequest) (map[string]operatorv1alpha1.ConfigService, error) {

	requestMap, err := r.fetchRequests(cr)
	if err != nil {
		return nil, err
	}

	cscMap := make(map[string]operatorv1alpha1.ConfigService)
	for k, v := range csc.Spec.Services {
		if _, ok := requestMap[v.Name]; ok {
			csc.Spec.Services[k].State = requestMap[v.Name].State
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

// getCSV retrieves the Cluster Service Version
func (r *ReconcileOperandRequest) getClusterServiceVersion(subName string) (*olmv1alpha1.ClusterServiceVersion, error) {
	logger := log.WithValues("Subscription Name", subName)
	logger.Info("Looking for the Cluster Service Version")
	subs, listSubErr := r.olmClient.OperatorsV1alpha1().Subscriptions("").List(metav1.ListOptions{
		LabelSelector: "operator.ibm.com/opreq-control",
	})
	if listSubErr != nil {
		logger.Error(listSubErr, "Fail to list subscriptions")
		return nil, listSubErr
	}
	var csvName, csvNamespace string
	for _, s := range subs.Items {
		if s.Name == subName {
			if s.Status.CurrentCSV == "" {
				logger.Info(fmt.Sprintf("There is no Cluster Service Version for %s", subName))
				return nil, nil
			}
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
	logger.Info(fmt.Sprintf("There is no Cluster Service Version for %s", subName))
	return nil, nil
}

// createUpdateCr merge and create custome resource base on OperandConfig and CSV alm-examples
func (r *ReconcileOperandRequest) createUpdateCr(service operatorv1alpha1.ConfigService, csv *olmv1alpha1.ClusterServiceVersion, csc *operatorv1alpha1.OperandConfig) error {
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

	// Merge OperandConfig and Cluster Service Version alm-examples
	for _, crTemplate := range crTemplates {

		// Create an unstruct object for CR and request its value to CR template
		var unstruct unstructured.Unstructured
		unstruct.Object = crTemplate.(map[string]interface{})

		// Get the kind of CR
		name := unstruct.Object["kind"]

		for crdName, crConfig := range service.Spec {

			// Compare the name of OperandConfig and CRD name
			if strings.EqualFold(name.(string), crdName) {
				logger.Info(fmt.Sprintf("Found OperandConfig spec for custom resource %s", name))
				//Convert CR template spec to string
				specJSONString, _ := json.Marshal(unstruct.Object["spec"])

				// Merge CR template spec and OperandConfig spec
				mergedCR := util.MergeCR(specJSONString, crConfig.Raw)

				unstruct.Object["spec"] = mergedCR
				unstruct.Object["metadata"].(map[string]interface{})["namespace"] = namespace

				// Creat or Update the CR
				crCreateErr := r.client.Create(context.TODO(), &unstruct)
				if crCreateErr != nil && !errors.IsAlreadyExists(crCreateErr) {
					stateUpdateErr := r.updateServiceStatus(csc, service.Name, crdName, operatorv1alpha1.ServiceFailed)
					if stateUpdateErr != nil {
						merr.Add(stateUpdateErr)
					}
					logger.Error(crCreateErr, "Fail to Create the Custom Resource "+crdName)
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
						logger.Error(crGetErr, "Fail to Get the Custom Resource "+crdName)
						merr.Add(crGetErr)
						continue
					}
					existingCR.Object["spec"] = unstruct.Object["spec"]
					if crUpdateErr := r.client.Update(context.TODO(), existingCR); crUpdateErr != nil {
						stateUpdateErr := r.updateServiceStatus(csc, service.Name, crdName, operatorv1alpha1.ServiceFailed)
						if stateUpdateErr != nil {
							merr.Add(stateUpdateErr)
						}
						logger.Error(crUpdateErr, "Fail to Update the Custom Resource "+crdName)
						merr.Add(crUpdateErr)
						continue
					}
					logger.Info("Finish updating the Custom Resource: " + crdName)
					stateUpdateErr := r.updateServiceStatus(csc, service.Name, crdName, operatorv1alpha1.ServiceRunning)
					if stateUpdateErr != nil {
						merr.Add(stateUpdateErr)
					}

				} else {
					logger.Info("Finish creating the Custom Resource " + crdName)
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

// deleteCr remove custome resource base on OperandConfig and CSV alm-examples
func (r *ReconcileOperandRequest) deleteCr(service operatorv1alpha1.ConfigService, csv *olmv1alpha1.ClusterServiceVersion, csc *operatorv1alpha1.OperandConfig) error {
	almExamples := csv.ObjectMeta.Annotations["alm-examples"]
	logger := log.WithValues("Subscription", service.Name)
	namespace := csv.ObjectMeta.Namespace

	// Create a slice for crTemplates
	var crTemplates []interface{}

	// Convert CR template string to slice
	crTemplatesErr := json.Unmarshal([]byte(almExamples), &crTemplates)
	if crTemplatesErr != nil {
		logger.Error(crTemplatesErr, "Fail to convert alm-examples to slice")
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
				crDeleteErr := r.client.DeleteAllOf(context.TODO(), &unstruct)
				if crDeleteErr != nil {
					merr.Add(crDeleteErr)
					continue
				}

				logger.Info("Waiting for CR: " + kind + " is deleted")
				stateDeleteErr := r.deleteServiceStatus(csc, service.Name, crdName)
				if stateDeleteErr != nil {
					merr.Add(stateDeleteErr)
				}
				err := wait.PollImmediate(time.Second*20, time.Minute*10, func() (bool, error) {
					logger.Info("Checking for CR: " + kind + " is deleted")
					err := r.client.Get(context.TODO(), types.NamespacedName{
						Name:      name,
						Namespace: namespace,
					},
						&unstruct)
					if errors.IsNotFound(err) {
						return true, nil
					}
					if err != nil {
						return false, err
					}
					return false, nil
				})
				if err != nil {
					merr.Add(err)
				}
				logger.Info("Deleted the CR: " + kind)
			}

		}
	}
	if len(merr.errors) != 0 {
		return merr
	}

	return nil
}
