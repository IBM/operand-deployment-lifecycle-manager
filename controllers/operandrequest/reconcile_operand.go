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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"

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
		regName := registryInstance.ObjectMeta.Name
		regNs := registryInstance.ObjectMeta.Namespace

		for i, operand := range req.Operands {

			opdRegistry := registryInstance.GetOperator(operand.Name)
			if opdRegistry == nil {
				klog.Warningf("Cannot find %s in the OperandRegistry instance %s in the namespace %s ", operand.Name, req.Registry, req.RegistryNamespace)
				continue
			}

			operatorName := opdRegistry.Name

			klog.V(3).Info("Looking for csv for the operator: ", operatorName)

			// Looking for the CSV
			namespace := r.GetOperatorNamespace(opdRegistry.InstallMode, opdRegistry.Namespace)

			sub, err := r.GetSubscription(ctx, operatorName, namespace, opdRegistry.PackageName)

			if err != nil {
				if apierrors.IsNotFound(err) || sub == nil {
					klog.Warningf("There is no Subscription %s or %s in the namespace %s", operatorName, opdRegistry.PackageName, namespace)
					continue
				}
				merr.Add(errors.Wrapf(err, "failed to get the Subscription %s in the namespace %s", operatorName, namespace))
				return merr
			}

			if _, ok := sub.Labels[constant.OpreqLabel]; !ok {
				// Subscription existing and not managed by OperandRequest controller
				klog.Warningf("Subscription %s in the namespace %s isn't created by ODLM", sub.Name, sub.Namespace)
			}

			// check config annotation in subscription, identify the first ODLM has the priority to reconcile
			var firstMatch string
			reg, _ := regexp.Compile(`^(.*)\.(.*)\/config`)
			for anno := range sub.Annotations {
				if reg.MatchString(anno) {
					firstMatch = anno
					break
				}
			}

			if firstMatch != "" && firstMatch != regNs+"."+regName+"/config" {
				klog.V(2).Infof("Subscription %s in the namespace %s is currently managed by %s", sub.Name, sub.Namespace, firstMatch)
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

			if csv.Status.Phase == olmv1alpha1.CSVPhaseFailed {
				merr.Add(fmt.Errorf("the ClusterServiceVersion of Subscription %s/%s is Failed", namespace, operatorName))
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorFailed, "", &r.Mutex)
				continue
			}
			if csv.Status.Phase != olmv1alpha1.CSVPhaseSucceeded {
				klog.Errorf("the ClusterServiceVersion of Subscription %s/%s is not Ready", namespace, operatorName)
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorInstalling, "", &r.Mutex)
				continue
			}

			klog.V(3).Info("Generating customresource base on ClusterServiceVersion: ", csv.GetName())
			requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorRunning, "", &r.Mutex)
			klog.Info(requestInstance.Status.Members)

			// Merge and Generate CR
			if operand.Kind == "" {
				configInstance, err := r.GetOperandConfig(ctx, registryKey)
				if err != nil {
					merr.Add(errors.Wrapf(err, "failed to get the OperandConfig %s", registryKey.String()))
					continue
				}
				// Check the requested Service Config if exist in specific OperandConfig
				opdConfig := configInstance.GetService(operand.Name)
				if opdConfig == nil {
					klog.V(2).Infof("There is no service: %s from the OperandConfig instance: %s/%s, Skip creating CR for it", operand.Name, req.RegistryNamespace, req.Registry)
					continue
				}
				err = r.reconcileCRwithConfig(ctx, opdConfig, opdRegistry.Namespace, csv)
				if err != nil {
					merr.Add(err)
					requestInstance.SetMemberStatus(operand.Name, "", operatorv1alpha1.ServiceFailed, &r.Mutex)
				}
			} else {
				err = r.reconcileCRwithRequest(ctx, requestInstance, operand, types.NamespacedName{Name: requestInstance.Name, Namespace: requestInstance.Namespace}, i)
				if err != nil {
					merr.Add(err)
					requestInstance.SetMemberStatus(operand.Name, "", operatorv1alpha1.ServiceFailed, &r.Mutex)
				}
			}
			requestInstance.SetMemberStatus(operand.Name, "", operatorv1alpha1.ServiceRunning, &r.Mutex)
			klog.Info(requestInstance.Status.Members)
		}
	}
	if len(merr.Errors) != 0 {
		return merr
	}
	klog.V(1).Infof("Finished reconciling Operands for OperandRequest: %s/%s", requestInstance.GetNamespace(), requestInstance.GetName())
	return &util.MultiErr{}
}

// reconcileCRwithConfig merge and create custom resource base on OperandConfig and CSV alm-examples
func (r *Reconciler) reconcileCRwithConfig(ctx context.Context, service *operatorv1alpha1.ConfigService, namespace string, csv *olmv1alpha1.ClusterServiceVersion) error {
	almExamples := csv.GetAnnotations()["alm-examples"]

	// Convert CR template string to slice
	var almExampleList []interface{}
	err := json.Unmarshal([]byte(almExamples), &almExampleList)
	if err != nil {
		return errors.Wrapf(err, "failed to convert alm-examples in the Subscription %s/%s to slice", namespace, service.Name)
	}

	merr := &util.MultiErr{}

	foundMap := make(map[string]bool)
	for cr := range service.Spec {
		foundMap[cr] = false
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
			Namespace: namespace,
		}, &crFromALM)

		for cr := range service.Spec {
			if strings.EqualFold(crFromALM.GetKind(), cr) {
				foundMap[cr] = true
			}
		}

		if err != nil && !apierrors.IsNotFound(err) {
			merr.Add(errors.Wrapf(err, "failed to get the custom resource %s/%s", namespace, name))
			continue
		} else if apierrors.IsNotFound(err) {
			// Create Custom Resource
			if err := r.compareConfigandExample(ctx, crFromALM, service, namespace); err != nil {
				merr.Add(err)
				continue
			}
		} else {
			if checkLabel(crFromALM, map[string]string{constant.OpreqLabel: "true"}) {
				// Update or Delete Custom Resource
				if err := r.existingCustomResource(ctx, crFromALM, spec.(map[string]interface{}), service, namespace); err != nil {
					merr.Add(err)
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

	for cr, found := range foundMap {
		if !found {
			klog.Warningf("Custom resource %v doesn't exist in the alm-example of %v", cr, csv.GetName())
		}
	}

	return nil
}

// reconcileCRwithRequest merge and create custom resource base on OperandRequest and CSV alm-examples
func (r *Reconciler) reconcileCRwithRequest(ctx context.Context, requestInstance *operatorv1alpha1.OperandRequest, operand operatorv1alpha1.Operand, requestKey types.NamespacedName, index int) error {
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
		if checkLabel(crFromRequest, map[string]string{constant.OpreqLabel: "true"}) {
			// Update or Delete Custom resource
			klog.V(3).Info("Found existing custom resource: " + operand.Kind)
			if err := r.updateCustomResource(ctx, crFromRequest, requestKey.Namespace, operand.Kind, operand.Spec.Raw, map[string]interface{}{}); err != nil {
				return err
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
				if checkLabel(crTemplate, map[string]string{constant.OpreqLabel: "true"}) {
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

	ensureLabel(crTemplate, map[string]string{constant.OpreqLabel: "true"})

	// Creat the CR
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

func (r *Reconciler) updateCustomResource(ctx context.Context, existingCR unstructured.Unstructured, namespace, crName string, crConfig []byte, configFromALM map[string]interface{}) error {

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

		if !checkLabel(existingCR, map[string]string{constant.OpreqLabel: "true"}) {
			return true, nil
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

		if reflect.DeepEqual(existingCR.Object["spec"], updatedCRSpec) {
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
		if checkLabel(crShouldBeDeleted, map[string]string{constant.OpreqLabel: "true"}) && !checkLabel(crShouldBeDeleted, map[string]string{constant.NotUninstallLabel: "true"}) {
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

func checkLabel(unstruct unstructured.Unstructured, labels map[string]string) bool {
	for k, v := range labels {
		if !hasLabel(unstruct, k) {
			return false
		}
		if unstruct.GetLabels()[k] != v {
			return false
		}
	}
	return true
}

func hasLabel(cr unstructured.Unstructured, labelName string) bool {
	if cr.GetLabels() == nil {
		return false
	}
	if _, ok := cr.GetLabels()[labelName]; !ok {
		return false
	}
	return true
}

func ensureLabel(cr unstructured.Unstructured, labels map[string]string) bool {
	if cr.GetLabels() == nil {
		cr.SetLabels(make(map[string]string))
	}
	existingLabels := cr.GetLabels()
	for k, v := range labels {
		existingLabels[k] = v
	}
	cr.SetLabels(existingLabels)
	return true
}
