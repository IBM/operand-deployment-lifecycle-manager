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
	"context"
	"fmt"
	"os"
	"time"

	"github.com/operator-framework/operator-lifecycle-manager/pkg/lib/ownerutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/util"
)

// CSWebhookConfig contains the data and logic to setup the webhooks
// server of a given Manager implementation, and to reconcile webhook configuration
// CRs pointing to the server.
type CSWebhookConfig struct {
	scheme *runtime.Scheme

	Port        int
	CertDir     string
	CAConfigMap string

	Webhooks []CSWebhook
}

// CSWebhook acts as a single source of truth for validating webhooks
// managed by the operator. It's data are used both for registering the
// endpoint to the webhook server and to reconcile the ValidatingWebhookConfiguration
// that points to the server.
type CSWebhook struct {
	// Name of the webhookConfiguration.
	Name string

	// Name of the webhook.
	WebhookName string

	// Rule for the webhook to be triggered
	Rule RuleWithOperations

	// Register for the webhook into the server
	Register WebhookRegister

	// NsSelector for add namespaceselector to the admission webhook
	NsSelector v1.LabelSelector
}

const (
	operatorPodServiceName = "operand-deployment-lifecycle-manager"
	operatorPodPort        = 8443
	servicePort            = 443
	mountedCertDir         = "/etc/ssl/certs/webhook"
	caConfigMap            = "operand-deployment-lifecycle-manager-webhook-ca"
	caConfigMapAnnotation  = "service.beta.openshift.io/inject-cabundle"
	caServiceAnnotation    = "service.beta.openshift.io/serving-cert-secret-name"
	caCertificateName      = "odlm-webhook-cert"
)

var labels = map[string]string{
	constant.OdlmManagedLabel:      "true",
	"app.kubernetes.io/instance":   constant.OperatorName,
	"app.kubernetes.io/managed-by": constant.OperatorName,
	"app.kubernetes.io/name":       constant.OperatorName,
	"name":                         constant.OperatorName,
}

// Config is a global instance. The same instance is needed in order to use the
// same configuration for the webhooks server that's run at startup and the
// reconciliation of the ValidatingWebhookConfiguration CRs
var Config *CSWebhookConfig = &CSWebhookConfig{
	// Port that the webhook service is pointing to
	Port: operatorPodPort,

	// Mounted as a volume from the secret generated from Openshift
	CertDir: mountedCertDir,

	// Name of the config map where the CA certificate is injected
	CAConfigMap: caConfigMap,

	// List of webhooks to configure
	Webhooks: []CSWebhook{},
}

// SetupServer sets up the webhook server managed by mgr with the settings from
// webhookConfig. It sets the port and cert dir based on the settings and
// registers the Validator implementations from each webhook from webhookConfig.Webhooks
func (webhookConfig *CSWebhookConfig) SetupServer(mgr manager.Manager, namespace string) error {
	// Create a new client to reconcile the Service. `mgr.GetClient()` can't
	// be used as it relies on the cache that hasn't been initialized yet
	client, err := k8sclient.New(mgr.GetConfig(), k8sclient.Options{
		Scheme: mgr.GetScheme(),
	})
	if err != nil {
		return err
	}

	// Create the service pointing to the operator pod
	if err := webhookConfig.ReconcileService(context.TODO(), client, nil, namespace); err != nil {
		return err
	}
	// Get the secret with the certificates for the service
	if err := webhookConfig.setupCerts(context.TODO(), client, namespace); err != nil {
		return err
	}

	webhookServer := mgr.GetWebhookServer()
	webhookServer.Port = webhookConfig.Port
	webhookServer.CertDir = webhookConfig.CertDir

	webhookConfig.scheme = mgr.GetScheme()

	bldr := builder.WebhookManagedBy(mgr)

	for _, webhook := range webhookConfig.Webhooks {
		bldr = webhook.Register.RegisterToBuilder(bldr)
		if err := webhook.Register.RegisterToServer(webhookConfig.scheme, webhookServer); err != nil {
			return err
		}
	}

	if err := bldr.Complete(); err != nil {
		return err
	}

	return nil
}

// Reconcile reconciles a `ValidationWebhookConfiguration` object for each webhook
// in `webhookConfig.Webhooks`, using the rules and the path as it's generated
// by controller-runtime webhook builder.
// It reconciles a Service that exposes the webhook server
// A ownerRef to the owner parameter is set on the reconciled resources. This
// parameter is optional, if `nil` is passed, no ownerReference will be set
func (webhookConfig *CSWebhookConfig) Reconcile(ctx context.Context, client k8sclient.Client, owner ownerutil.Owner) error {

	namespace := util.GetOperatorNamespace()

	// Reconcile the Service
	if err := webhookConfig.ReconcileService(ctx, client, owner, namespace); err != nil {
		return err
	}

	// Create (if it doesn't exist) the config map where the CA certificate is
	// injected
	caConfigMap := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      webhookConfig.CAConfigMap,
			Namespace: namespace,
			Annotations: map[string]string{
				caConfigMapAnnotation: "true",
			},
		},
	}
	if owner != nil {
		ownerutil.EnsureOwner(caConfigMap, owner)
	}

	klog.Info("Creating operand deployment lifecycle manager webhook CA ConfigMap")
	if err := client.Create(ctx, caConfigMap); err != nil && !errors.IsAlreadyExists(err) {
		klog.Error(err)
		return err
	}

	// Wait for the config map to be injected with the CA
	caBundle, err := webhookConfig.waitForCAInConfigMap(ctx, client, namespace)
	if err != nil {
		klog.Error(err)
		return err
	}

	// Reconcile the webhooks
	for _, webhook := range webhookConfig.Webhooks {
		reconciler, err := webhook.Register.GetReconciler(webhookConfig.scheme)
		if err != nil {
			return err
		}

		reconciler.SetName(webhook.Name)
		reconciler.SetWebhookName(webhook.WebhookName)
		reconciler.SetRule(webhook.Rule)
		reconciler.SetNsSelector(webhook.NsSelector)
		klog.Infof("Reconciling webhook %s", webhook.Name)
		if err := reconciler.Reconcile(ctx, client, caBundle); err != nil {
			return err
		}
	}

	return nil
}

// ReconcileService creates or updates the service that points to the Pod
func (webhookConfig *CSWebhookConfig) ReconcileService(ctx context.Context, client k8sclient.Client, owner ownerutil.Owner, namespace string) error {

	klog.Info("Reconciling operand deployment lifecycle manager webhook service")
	// Get the service. If it's not found, create it
	service := &corev1.Service{}
	if err := client.Get(ctx, k8sclient.ObjectKey{
		Namespace: namespace,
		Name:      operatorPodServiceName,
	}, service); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		return createService(ctx, client, owner, namespace)
	}

	// If the existing service has a different .spec.clusterIP value, delete it
	if service.Spec.ClusterIP != "None" {
		if err := client.Delete(ctx, service); err != nil {
			return err
		}
	}

	return createService(ctx, client, owner, namespace)
}

func createService(ctx context.Context, client k8sclient.Client, owner ownerutil.Owner, namespace string) error {
	klog.Info("Creating operand deployment lifecycle manager webhook service")

	service := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:      operatorPodServiceName,
			Namespace: namespace,
			Labels:    labels,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, client, service, func() error {
		if owner != nil {
			ownerutil.EnsureOwner(service, owner)
		}

		if service.Annotations == nil {
			service.Annotations = map[string]string{}
		}
		service.Annotations[caServiceAnnotation] = caCertificateName
		service.Spec.ClusterIP = "None"
		service.Spec.Selector = map[string]string{
			"name": constant.OperatorName,
		}
		service.Spec.Ports = []corev1.ServicePort{
			{
				Protocol:   corev1.ProtocolTCP,
				Port:       int32(servicePort),
				TargetPort: intstr.FromInt(operatorPodPort),
			},
		}

		return nil
	})
	if err != nil {
		klog.Error(err)
	}
	return err
}

// setupCerts waits for the secret created for the operator Service to exist, and
// when it's ready, extracts the certificates and saves them in webhookConfig.CertDir
func (webhookConfig *CSWebhookConfig) setupCerts(ctx context.Context, client k8sclient.Client, namespace string) error {
	// Wait for the secret to te created
	secret := &corev1.Secret{}
	err := wait.PollImmediate(time.Second*1, time.Second*30, func() (bool, error) {
		err := client.Get(ctx, k8sclient.ObjectKey{Namespace: namespace, Name: caCertificateName}, secret)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		return true, nil
	})
	if err != nil {
		return err
	}

	// Save the key
	if err := webhookConfig.saveCertFromSecret(secret.Data, "tls.key"); err != nil {
		return err
	}
	// Save the cert
	return webhookConfig.saveCertFromSecret(secret.Data, "tls.crt")
}

func (webhookConfig *CSWebhookConfig) waitForCAInConfigMap(ctx context.Context, client k8sclient.Client, namespace string) ([]byte, error) {
	klog.Info("Waiting for operand deployment lifecycle manager webhook CA generated")

	var caBundle []byte

	err := wait.PollImmediate(time.Second, time.Second*30, func() (bool, error) {
		caConfigMap := &corev1.ConfigMap{}
		if err := client.Get(ctx,
			k8sclient.ObjectKey{Name: webhookConfig.CAConfigMap, Namespace: namespace},
			caConfigMap,
		); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}

			return false, err
		}

		result, ok := caConfigMap.Data["service-ca.crt"]

		if !ok {
			return false, nil
		}

		caBundle = []byte(result)
		return true, nil
	})

	return caBundle, err
}

// AddWebhook adds a webhook configuration to a webhookSettings. This must be done before
// starting the server as it registers the endpoints for the validation
func (webhookConfig *CSWebhookConfig) AddWebhook(webhook CSWebhook) {
	webhookConfig.Webhooks = append(webhookConfig.Webhooks, webhook)
}

func (webhookConfig *CSWebhookConfig) saveCertFromSecret(secretData map[string][]byte, fileName string) error {
	value, ok := secretData[fileName]
	if !ok {
		return fmt.Errorf("secret does not contain key %s", fileName)
	}

	// Save the key
	f, err := os.Create(fmt.Sprintf("%s/%s", webhookConfig.CertDir, fileName))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(value)
	return err
}
