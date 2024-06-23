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
	"net/http"
	"strings"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	odlm "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
)

// OperandRequestDefaulter points to correct RegistryNamespace
type Defaulter struct {
	decoder    *admission.Decoder
	Client     client.Client
	OperatorNs string
}

func (r *Defaulter) Handle(ctx context.Context, req admission.Request) admission.Response {
	klog.Infof("Webhook is invoked by OperandRequest %s/%s", req.AdmissionRequest.Namespace, req.AdmissionRequest.Name)

	if req.AdmissionRequest.Operation == admissionv1.Create {
		return admission.Allowed("")
	}

	isDeleting := req.AdmissionRequest.Operation == admissionv1.Delete

	if req.AdmissionRequest.Operation == admissionv1.Update {
		opreq := &odlm.OperandRequest{}

		if err := r.decoder.Decode(req, opreq); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		// Check if the OperandRequest is being deleted
		if opreq.DeletionTimestamp != nil {
			isDeleting = true
		} else {
			// check if the newObject contains annotation "operand.ibm.com/watched-by-odlm-in-$OperatorNs" key and it is not empty value nor false
			if opreq.Annotations == nil || opreq.Annotations["operand.ibm.com/watched-by-odlm-in-"+r.OperatorNs] == "false" || opreq.Annotations["operand.ibm.com/watched-by-odlm-in-"+r.OperatorNs] == "" {
				klog.Warningf("OperandRequest %s is not watched by ODLM in the %s namespace", opreq.Name, r.OperatorNs)
				return admission.Allowed("")
			}

			// get the new OperandRequest spec with the valid operands
			if newOpreqSpec, err := GetFilteredOpreqSpec(ctx, r.Client, opreq, r.OperatorNs); err != nil {
				klog.Errorf("Failed to get new OperandRequest spec: %v", err)
				return admission.Errored(http.StatusInternalServerError, err)
			} else if newOpreqSpec == nil || len(newOpreqSpec.Requests) == 0 {
				klog.Infof("No valid operand found for OperandRequest %s in the %s namespace to replicate, deleting it", opreq.Name, r.OperatorNs)
				isDeleting = true
			} else {
				existingOpreq := &odlm.OperandRequest{}
				// get the OperandRequest from the r.OperatorNs namespace
				opreqKey := types.NamespacedName{
					Name:      opreq.Name + "-from-" + opreq.Namespace,
					Namespace: r.OperatorNs,
				}
				if err := r.Client.Get(ctx, opreqKey, existingOpreq); err != nil {
					if errors.IsNotFound(err) {
						// if the OperandRequest is not found, create a new OperandRequest in the r.OperatorNs namespace
						klog.Infof("OperandRequest %s not found in the %s namespace", opreq.Name, r.OperatorNs)
						existingOpreq.Name = opreq.Name + "-from-" + opreq.Namespace
						existingOpreq.Namespace = r.OperatorNs
						existingOpreq.Spec = *newOpreqSpec

						// Add label "operand.ibm.com/opreq-replicated-from-$opreq.Namespace" to the OperandRequest
						if existingOpreq.Labels == nil {
							existingOpreq.Labels = make(map[string]string)
						}
						existingOpreq.Labels["operand.ibm.com/opreq-replicated-from-"+opreq.Namespace] = "true"

						if err = r.Client.Create(ctx, existingOpreq); err != nil {
							klog.Errorf("Failed to replicate OperandRequest %s in the %s namespace: %v", existingOpreq.Name, r.OperatorNs, err)
							return admission.Errored(http.StatusInternalServerError, err)
						}
						klog.Infof("OperandRequest %s is replicated in the %s namespace", existingOpreq.Name, r.OperatorNs)
						return admission.Allowed("")
					}
					klog.Errorf("Failed to get OperandRequest %s in the %s namespace: %v", existingOpreq.Name, r.OperatorNs, err)
					return admission.Errored(http.StatusInternalServerError, err)
				}
				// update the existing OperandRequest in the r.OperatorNs namespace
				existingOpreq.Spec = *newOpreqSpec
				if err := r.Client.Update(ctx, existingOpreq); err != nil {
					klog.Errorf("Failed to update OperandRequest %s in the %s namespace: %v", existingOpreq.Name, r.OperatorNs, err)
					return admission.Errored(http.StatusInternalServerError, err)
				}
				klog.Infof("OperandRequest %s is updated in the %s namespace", existingOpreq.Name, r.OperatorNs)
			}
		}
	}

	if isDeleting {
		// invoke a delete action on the OperandRequest named opreq.Name-from-opreq.Namespace in the r.OperatorNs namespace
		existingOpreq := &odlm.OperandRequest{}
		// get the OperandRequest from the r.OperatorNs namespace
		opreqKey := types.NamespacedName{
			Name:      req.Name + "-from-" + req.Namespace,
			Namespace: r.OperatorNs,
		}
		if err := r.Client.Get(ctx, opreqKey, existingOpreq); err != nil {
			if errors.IsNotFound(err) {
				klog.Infof("OperandRequest %s not found in the %s namespace", existingOpreq.Name, r.OperatorNs)
				return admission.Allowed("")
			}
			klog.Errorf("Failed to get OperandRequest %s in the %s namespace: %v", existingOpreq.Name, r.OperatorNs, err)
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if err := r.Client.Delete(ctx, existingOpreq); err != nil {
			klog.Errorf("Failed to delete OperandRequest %s in the %s namespace: %v", existingOpreq.Name, r.OperatorNs, err)
			return admission.Errored(http.StatusInternalServerError, err)
		}
		klog.Infof("OperandRequest %s is deleted in the %s namespace", existingOpreq.Name, r.OperatorNs)
		return admission.Allowed("")
	}

	return admission.Allowed("")
}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Defaulter) Default(instance *odlm.OperandRequest) {
}

func GetFilteredOpreqSpec(ctx context.Context, kube client.Client, opreq *odlm.OperandRequest, operatorNs string) (*odlm.OperandRequestSpec, error) {
	newOpreqSpec := &odlm.OperandRequestSpec{}
	for _, req := range opreq.Spec.Requests {
		// get the OperandRegistry from the registry and registryNamespace
		registryKey := types.NamespacedName{
			Name:      req.Registry,
			Namespace: operatorNs,
		}
		registry := &odlm.OperandRegistry{}
		if err := kube.Get(ctx, registryKey, registry); err != nil {
			if errors.IsNotFound(err) {
				klog.Warningf("OperandRegistry %s not found in the %s namespace", registryKey.Name, registryKey.Namespace)
				continue
			}
			klog.Errorf("Failed to get OperandRegistry %s in the %s namespace: %v", registryKey.Name, registryKey.Namespace, err)
			return nil, err
		}
		newOpreqSpecRequest := odlm.Request{Registry: req.Registry, RegistryNamespace: operatorNs}
		for _, op := range req.Operands {
			opt := registry.GetOperator(op.Name)
			if opt != nil {
				newOpreqSpecRequest.Operands = append(newOpreqSpecRequest.Operands, op)
			}
		}
		if len(newOpreqSpecRequest.Operands) > 0 {
			newOpreqSpec.Requests = append(newOpreqSpec.Requests, newOpreqSpecRequest)
		}
	}
	return newOpreqSpec, nil
}

func (r *Defaulter) InjectDecoder(decoder *admission.Decoder) error {
	r.decoder = decoder
	return nil
}

func AddAnnotationToOperandRequests(kube client.Client, partialWatchNamespace, operatorNamespace string) {
	for {
		// wait for OperandRegistry common-service in operatorNamespace to be created
		registryKey := types.NamespacedName{
			Name:      "common-service",
			Namespace: operatorNamespace,
		}
		registry := &odlm.OperandRegistry{}
		if err := kube.Get(context.TODO(), registryKey, registry); err != nil {
			klog.Warningf("Failed to get OperandRegistry common-service in the %s namespace: %v, retrying...", operatorNamespace, err)
			time.Sleep(5 * time.Second)
			continue
		}

		isFinished := true
		sleepTime := 5 * time.Second
		// Get all OperandRequests in the partial watch namespace
		for _, ns := range strings.Split(partialWatchNamespace, ",") {
			opreqList := &odlm.OperandRequestList{}
			if err := kube.List(context.TODO(), opreqList, &client.ListOptions{Namespace: ns}); err != nil {
				isFinished = false
				klog.Warningf("Failed to list OperandRequests in the %s namespace: %v", ns, err)
				continue
			}

			for _, opreq := range opreqList.Items {
				opreq := opreq
				annotations := opreq.GetAnnotations()
				if annotations == nil {
					annotations = make(map[string]string)
				}
				// if annotation value is false, then skip it
				if annotations["operand.ibm.com/watched-by-odlm-in-"+operatorNamespace] == "false" {
					continue
				}
				// Always refresh the annotation value to trigger the webhook replication
				annotations["operand.ibm.com/watched-by-odlm-in-"+operatorNamespace] = time.Now().Format(time.RFC3339)
				opreq.SetAnnotations(annotations)
				if err := kube.Update(context.Background(), &opreq); err != nil {
					isFinished = false
					klog.Warningf("Failed to add annotation to OperandRequest %s/%s: %v", ns, opreq.GetName(), err)
					continue
				}

				// Check if there are valid operand in the OperandRequest requiring replication
				if newOpreqSpec, err := GetFilteredOpreqSpec(context.TODO(), kube, &opreq, operatorNamespace); err != nil {
					isFinished = false
					klog.Errorf("Failed to get new OperandRequest spec: %v", err)
					continue
				} else if newOpreqSpec == nil || len(newOpreqSpec.Requests) == 0 {
					klog.Infof("No valid operand found for OperandRequest %s in the %s namespace to replicate, skipping it", opreq.Name, operatorNamespace)
				} else {
					// check the OperandRequest opreq.Name-from-opreq.Namespace is replicated in operatorNamespace
					opreqKey := types.NamespacedName{
						Namespace: operatorNamespace,
						Name:      opreq.GetName() + "-from-" + ns,
					}
					opreqInOperatorNs := &odlm.OperandRequest{}
					if err := kube.Get(context.TODO(), opreqKey, opreqInOperatorNs); err != nil {
						isFinished = false
						klog.Warningf("Failed to get OperandRequest %s in the %s namespace: %v", opreqKey.Name, operatorNamespace, err)
						continue
					}
				}
			}
		}
		if isFinished {
			klog.Info("Successfully added annotation to all OperandRequests in the partial watch namespace")
			break
		} else {
			time.Sleep(sleepTime)
		}
	}
}
