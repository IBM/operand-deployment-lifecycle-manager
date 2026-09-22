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
	"net/http/httptest"
	"testing"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	"github.com/stretchr/testify/require"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operator"
)

// Package manifests can expose the same package from several catalogs. Keep their
// discovery response separate from the fake client's uniquely named objects.
type catalogTestReader struct {
	client.Reader
	manifests []operatorsv1.PackageManifest
}

func (r *catalogTestReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if manifests, ok := list.(*operatorsv1.PackageManifestList); ok {
		manifests.Items = r.manifests
		return nil
	}
	return r.Reader.List(ctx, list, opts...)
}

func TestReconcileSubscriptionCatalog(t *testing.T) {
	t.Setenv("NO_OLM", "false")
	t.Setenv("OPERATOR_NAMESPACE", "tenant")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api":
			_, _ = w.Write([]byte(`{"kind":"APIVersions","apiVersion":"v1","versions":["v1"]}`))
		case "/apis":
			_, _ = w.Write([]byte(`{"kind":"APIGroupList","apiVersion":"v1","groups":[{"name":"operators.coreos.com","versions":[{"groupVersion":"operators.coreos.com/v1alpha1","version":"v1alpha1"}],"preferredVersion":{"groupVersion":"operators.coreos.com/v1alpha1","version":"v1alpha1"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	const oldChannel = "stable-v1.25"
	const newChannel = "stable-v1.26"
	for _, tt := range []struct {
		name                                                          string
		explicit, unavailable, holdChannel, unmanaged, noop, fallback bool
		targetChannel, wantChannel, wantSource, wantNamespace         string
	}{
		{name: "private outage preserves pinned source", targetChannel: oldChannel, wantChannel: oldChannel, wantSource: "pinned", wantNamespace: "tenant"},
		{name: "channel upgrade permits automatic source switch", targetChannel: newChannel, wantChannel: newChannel, wantSource: "global", wantNamespace: "openshift-marketplace"},
		{name: "another request retains current channel and source", holdChannel: true, targetChannel: newChannel, wantChannel: oldChannel, wantSource: "pinned", wantNamespace: "tenant"},
		{name: "fallback resolves to current channel", fallback: true, targetChannel: newChannel, wantChannel: oldChannel, wantSource: "pinned", wantNamespace: "tenant"},
		{name: "explicit same channel migration", explicit: true, targetChannel: oldChannel, wantChannel: oldChannel, wantSource: "global", wantNamespace: "openshift-marketplace"},
		{name: "unavailable explicit source never falls back", explicit: true, unavailable: true, targetChannel: oldChannel, wantChannel: oldChannel, wantSource: "pinned", wantNamespace: "tenant"},
		{name: "unmanaged subscription stays unchanged", unmanaged: true, targetChannel: newChannel, wantChannel: oldChannel, wantSource: "pinned", wantNamespace: "tenant"},
		{name: "maintenance subscription stays unchanged", noop: true, targetChannel: oldChannel, wantChannel: oldChannel, wantSource: "pinned", wantNamespace: "tenant"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, olmv1alpha1.AddToScheme(scheme))
			require.NoError(t, operatorv1alpha1.AddToScheme(scheme))
			sub := &olmv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{Name: "postgres", Namespace: "tenant", Labels: map[string]string{constant.OpreqLabel: "true"}},
				Spec:       &olmv1alpha1.SubscriptionSpec{Package: "postgres", Channel: oldChannel, CatalogSource: "pinned", CatalogSourceNamespace: "tenant", InstallPlanApproval: olmv1alpha1.ApprovalAutomatic},
			}
			if tt.unmanaged {
				sub.Labels = nil
			}
			if tt.holdChannel {
				sub.Annotations = map[string]string{"tenant.other.postgres/request": oldChannel}
			}
			kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sub).WithInterceptorFuncs(interceptor.Funcs{
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					// Exercise discovery without permission to read catalog priorities.
					if _, ok := obj.(*authorizationv1.SelfSubjectAccessReview); ok {
						return nil
					}
					return c.Create(ctx, obj, opts...)
				},
			}).Build()
			reader := &catalogTestReader{Reader: kube, manifests: []operatorsv1.PackageManifest{{
				ObjectMeta: metav1.ObjectMeta{Name: "postgres", Namespace: "tenant"},
				Status:     operatorsv1.PackageManifestStatus{PackageName: "postgres", CatalogSource: "global", CatalogSourceNamespace: "openshift-marketplace", Channels: []operatorsv1.PackageChannel{{Name: oldChannel}, {Name: newChannel}}},
			}}}
			r := &Reconciler{ODLMOperator: &deploy.ODLMOperator{Client: kube, Reader: reader, Config: &rest.Config{Host: server.URL}}}
			reg := &operatorv1alpha1.OperandRegistry{ObjectMeta: metav1.ObjectMeta{Name: "common-service", Namespace: "tenant"}, Spec: operatorv1alpha1.OperandRegistrySpec{Operators: []operatorv1alpha1.Operator{{
				Name: "postgres", PackageName: "postgres", Namespace: "tenant", Channel: tt.targetChannel, InstallPlanApproval: olmv1alpha1.ApprovalManual,
				SubscriptionConfig: &olmv1alpha1.SubscriptionConfig{Env: []corev1.EnvVar{{Name: "TEST_CONFIG", Value: "updated"}}},
			}}}}
			if tt.fallback {
				reg.Spec.Operators[0].FallbackChannels = []string{oldChannel}
				reader.manifests[0].Status.Channels = []operatorsv1.PackageChannel{{Name: oldChannel}}
			}
			if tt.explicit {
				reg.Spec.Operators[0].SourceName = "global"
				reg.Spec.Operators[0].SourceNamespace = "openshift-marketplace"
				if tt.unavailable {
					reg.Spec.Operators[0].SourceName = "missing"
				}
			}
			if tt.noop {
				reg.Spec.Operators[0].InstallMode = operatorv1alpha1.InstallModeNoop
			}
			req := &operatorv1alpha1.OperandRequest{ObjectMeta: metav1.ObjectMeta{Name: "request", Namespace: "tenant"}}
			regKey := types.NamespacedName{Name: reg.Name, Namespace: reg.Namespace}
			reconcile := func() {
				require.NoError(t, r.reconcileSubscription(ctx, req, reg, operatorv1alpha1.Operand{Name: "postgres"}, regKey, &r.Mutex))
				got := &olmv1alpha1.Subscription{}
				require.NoError(t, kube.Get(ctx, types.NamespacedName{Name: sub.Name, Namespace: sub.Namespace}, got))
				require.Equal(t, tt.wantChannel, got.Spec.Channel)
				require.Equal(t, tt.wantSource, got.Spec.CatalogSource)
				require.Equal(t, tt.wantNamespace, got.Spec.CatalogSourceNamespace)
				if !tt.unavailable && !tt.holdChannel && !tt.unmanaged && !tt.noop {
					require.Equal(t, olmv1alpha1.ApprovalManual, got.Spec.InstallPlanApproval)
					require.Equal(t, reg.Spec.Operators[0].SubscriptionConfig, got.Spec.Config)
				}
			}
			reconcile()
			reconcile() // A second pass must preserve the same decision.
			if tt.name == "private outage preserves pinned source" {
				private := reader.manifests[0].DeepCopy()
				private.Status.CatalogSource = "pinned"
				private.Status.CatalogSourceNamespace = "tenant"
				reader.manifests = append(reader.manifests, *private)
				reconcile() // Recovery of the private catalog must also be stable.
			}
		})
	}
}
