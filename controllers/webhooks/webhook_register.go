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

package webhooks

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// WebhookRegister knows how the register a webhook into the server. Either by
// regstering to the WebhookBuilder or directly to the webhook server.
type WebhookRegister interface {
	RegisterToBuilder(blrd *builder.WebhookBuilder) *builder.WebhookBuilder
	RegisterToServer(scheme *runtime.Scheme, srv *webhook.Server) error

	GetReconciler(scheme *runtime.Scheme) (WebhookReconciler, error)
}

// ObjectWebhookRegister registers objects that implement either the `Validator`
// interface or the `Defaulting` interface into the WebhookBuilder
type ObjectWebhookRegister struct {
	Object runtime.Object
}

type valueForType struct {
	validating string
	mutating   string
}

// WebhookRegisterFor creates a WebhookRegister for a given object, validating
// beforehand that the object implements either the `Defaulter` of `Validator`
// interfaces
func WebhookRegisterFor(object runtime.Object) (*ObjectWebhookRegister, error) {
	_, isDefaulter := object.(admission.Defaulter)
	_, isValidator := object.(admission.Validator)

	if isDefaulter || isValidator {
		return &ObjectWebhookRegister{object}, nil
	}

	return nil, fmt.Errorf("object %v does not implement Defaulter or Validator interface", object)
}

// RegisterToBuilder adds the object into the builder, which registers the webhook
// for the object into the webhook server
func (vwr ObjectWebhookRegister) RegisterToBuilder(bldr *builder.WebhookBuilder) *builder.WebhookBuilder {
	return bldr.For(vwr.Object)
}

// RegisterToServer does nothing, as the register is done by the builder
func (vwr ObjectWebhookRegister) RegisterToServer(_ *runtime.Scheme, _ *webhook.Server) {}

// GetReconciler creates a reconciler according to the implementation of vwr.Object.
// The object can implement the `Validator` or `Defaulter` interfaces, and if both
// interfaces are implemented, two webhook configurations must be reconciled, as
// two endpoints will be registered in the webhook server
func (vwr ObjectWebhookRegister) GetReconciler(scheme *runtime.Scheme) (WebhookReconciler, error) {
	paths, err := vwr.getPaths(scheme)
	if err != nil {
		return nil, err
	}

	reconcilers := []WebhookReconciler{}

	if paths.mutating != "" {
		reconcilers = append(reconcilers, &MutatingWebhookReconciler{
			Path: paths.mutating,
		})
	}

	if paths.validating != "" {
		reconcilers = append(reconcilers, &ValidatingWebhookReconciler{
			Path: paths.validating,
		})
	}

	return &CompositeWebhookReconciler{
		Reconcilers: reconcilers,
	}, nil
}

// getPaths retrieves the paths for the webhook as implemented at controller-runtime/pkg/builder/webhook.go
// in order to match the path registered under the hood by the WebhookBuilder
func (vwr ObjectWebhookRegister) getPaths(scheme *runtime.Scheme) (*valueForType, error) {
	gvk, err := apiutil.GVKForObject(vwr.Object, scheme)
	if err != nil {
		return nil, err
	}

	result := &valueForType{}

	_, isDefaulter := vwr.Object.(admission.Defaulter)
	if isDefaulter {
		result.mutating = generatePath("mutate", gvk)
	}

	_, isValidator := vwr.Object.(admission.Validator)
	if isValidator {
		result.validating = generatePath("validate", gvk)
	}

	return result, nil
}

func generatePath(prefix string, gvk schema.GroupVersionKind) string {
	path := fmt.Sprintf("/%s-", prefix) + strings.Replace(gvk.Group, ".", "-", -1) + "-" +
		gvk.Version + "-" + strings.ToLower(gvk.Kind)

	return path
}

// WebhookType represents the type of webhook configuration to reconcile. Can
// be ValidatingType or MutatingType
type WebhookType string

// ValidatingType indicates that a ValidatingWebhookConfiguration must be
// reconciled
const ValidatingType = "Validating"

// MutatingType indicates that a MutatingWebhookConfiguration must be reconciled
const MutatingType = "Mutating"

// AdmissionWebhookRegister registers a given webhook into a specific path.
// This allows a more low level alternative to the WebhookBuilder, as it can
// directly get access the the AdmissionReview object sent to the webhook.
type AdmissionWebhookRegister struct {
	Type WebhookType
	Hook *admission.Webhook
	Path string
}

// RegisterToBuilder does not mutate the WebhookBuilder
func (awr AdmissionWebhookRegister) RegisterToBuilder(bldr *builder.WebhookBuilder) *builder.WebhookBuilder {
	return bldr
}

// RegisterToServer regsiters the webhook to the path of `awr`
func (awr AdmissionWebhookRegister) RegisterToServer(scheme *runtime.Scheme, srv *webhook.Server) error {
	err := awr.Hook.InjectScheme(scheme)
	if err != nil {
		return err
	}
	srv.Register(awr.Path, awr.Hook)
	return nil
}

// GetReconciler creates a reconciler for awr's given Path and Type
func (awr AdmissionWebhookRegister) GetReconciler(_ *runtime.Scheme) (WebhookReconciler, error) {
	switch awr.Type {
	case ValidatingType:
		return &ValidatingWebhookReconciler{
			Path: awr.Path,
		}, nil
	case MutatingType:
		return &MutatingWebhookReconciler{
			Path: awr.Path,
		}, nil
	}

	return nil, fmt.Errorf("unsupported type for AdmissionWebhookRegister: %s", awr.Type)
}
