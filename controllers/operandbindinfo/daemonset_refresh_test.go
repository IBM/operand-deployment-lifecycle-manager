//
// Copyright 2026 IBM Corporation
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

package operandbindinfo

import (
	"context"
	"errors"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operator"
)

const (
	refreshTestNamespace    = "test-namespace"
	refreshTestResourceName = "shared-secret"
	refreshTestResourceType = "secret"
)

type daemonSetListForbiddenReader struct {
	client.Reader
}

func (r daemonSetListForbiddenReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*appsv1.DaemonSetList); ok {
		return apierrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "daemonsets"}, "", errors.New("forbidden"))
	}
	return r.Reader.List(ctx, list, opts...)
}

type daemonSetUpdateForbiddenClient struct {
	client.Client
}

func (c daemonSetUpdateForbiddenClient) Update(ctx context.Context, object client.Object, opts ...client.UpdateOption) error {
	if daemonSet, ok := object.(*appsv1.DaemonSet); ok {
		return apierrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "daemonsets"}, daemonSet.Name, errors.New("forbidden"))
	}
	return c.Client.Update(ctx, object, opts...)
}

type recordingSSARClient struct {
	client.Client
	deniedVerb          string
	evaluationErrorVerb string
	createErrorVerb     string
	createError         error
	verbs               []string
}

func (c *recordingSSARClient) Create(ctx context.Context, object client.Object, opts ...client.CreateOption) error {
	review, ok := object.(*authorizationv1.SelfSubjectAccessReview)
	if !ok {
		return c.Client.Create(ctx, object, opts...)
	}

	verb := review.Spec.ResourceAttributes.Verb
	c.verbs = append(c.verbs, verb)
	if verb == c.createErrorVerb && c.createError != nil {
		return c.createError
	}
	review.Status.Allowed = verb != c.deniedVerb
	if verb == c.evaluationErrorVerb {
		review.Status.EvaluationError = "authorization backend unavailable"
	}
	return nil
}

func TestRefreshPodsSkipsDaemonSetsWithoutPermission(t *testing.T) {
	objects := refreshTestWorkloads()
	reconciler, k8sClient := newRefreshTestReconciler(t, objects, func(context.Context, string) (bool, string, error) {
		return false, "update permission was denied", nil
	})

	if err := reconciler.refreshPods(refreshTestNamespace, refreshTestResourceName, refreshTestResourceType); err != nil {
		t.Fatalf("refreshPods returned an error when DaemonSet permission was denied: %v", err)
	}

	assertRestarted(t, k8sClient, &appsv1.Deployment{}, "deployment", true)
	assertRestarted(t, k8sClient, &appsv1.StatefulSet{}, "statefulset", true)
	assertRestarted(t, k8sClient, &appsv1.DaemonSet{}, "daemonset", false)
}

func TestRefreshPodsFromDaemonSetWithPermission(t *testing.T) {
	objects := []client.Object{refreshTestDaemonSet()}
	reconciler, k8sClient := newRefreshTestReconciler(t, objects, func(context.Context, string) (bool, string, error) {
		return true, "", nil
	})

	if err := reconciler.refreshPodsFromDaemonSet(refreshTestNamespace, refreshTestResourceName, refreshTestResourceType); err != nil {
		t.Fatalf("refreshPodsFromDaemonSet returned an error with permission: %v", err)
	}
	assertRestarted(t, k8sClient, &appsv1.DaemonSet{}, "daemonset", true)
}

func TestRefreshPodsFromDaemonSetContinuesWhenListBecomesForbidden(t *testing.T) {
	objects := []client.Object{refreshTestDaemonSet()}
	reconciler, k8sClient := newRefreshTestReconciler(t, objects, func(context.Context, string) (bool, string, error) {
		return true, "", nil
	})
	reconciler.Reader = daemonSetListForbiddenReader{Reader: k8sClient}

	if err := reconciler.refreshPodsFromDaemonSet(refreshTestNamespace, refreshTestResourceName, refreshTestResourceType); err != nil {
		t.Fatalf("refreshPodsFromDaemonSet returned an error when DaemonSet list was forbidden: %v", err)
	}
	assertRestarted(t, k8sClient, &appsv1.DaemonSet{}, "daemonset", false)
}

func TestRefreshPodsFromDaemonSetContinuesWhenUpdateBecomesForbidden(t *testing.T) {
	objects := []client.Object{refreshTestDaemonSet()}
	reconciler, k8sClient := newRefreshTestReconciler(t, objects, func(context.Context, string) (bool, string, error) {
		return true, "", nil
	})
	reconciler.Client = daemonSetUpdateForbiddenClient{Client: k8sClient}

	if err := reconciler.refreshPodsFromDaemonSet(refreshTestNamespace, refreshTestResourceName, refreshTestResourceType); err != nil {
		t.Fatalf("refreshPodsFromDaemonSet returned an error when DaemonSet update was forbidden: %v", err)
	}
	assertRestarted(t, k8sClient, &appsv1.DaemonSet{}, "daemonset", false)
}

func TestCanManageDaemonSetsChecksRequiredPermissions(t *testing.T) {
	objects := []client.Object{refreshTestDaemonSet()}
	reconciler, k8sClient := newRefreshTestReconciler(t, objects, nil)
	recordingClient := &recordingSSARClient{Client: k8sClient}
	reconciler.Client = recordingClient

	allowed, reason, err := reconciler.canManageDaemonSets(context.Background(), refreshTestNamespace)
	if err != nil {
		t.Fatalf("expected DaemonSet permission check to succeed: %v", err)
	}
	if !allowed || reason != "" {
		t.Fatalf("expected DaemonSet access to be allowed, got allowed=%t reason=%q", allowed, reason)
	}
	if want := []string{"list", "update"}; !reflect.DeepEqual(recordingClient.verbs, want) {
		t.Fatalf("reviewed verbs = %v, want %v", recordingClient.verbs, want)
	}

	recordingClient.verbs = nil
	recordingClient.deniedVerb = "update"
	allowed, reason, err = reconciler.canManageDaemonSets(context.Background(), refreshTestNamespace)
	if err != nil {
		t.Fatalf("expected denied permission not to return an operational error: %v", err)
	}
	if allowed || reason == "" {
		t.Fatalf("expected update permission to be denied, got allowed=%t reason=%q", allowed, reason)
	}

	recordingClient.deniedVerb = ""
	for _, tc := range []struct {
		name      string
		createErr error
	}{
		{
			name: "forbidden",
			createErr: apierrors.NewForbidden(
				schema.GroupResource{Group: authorizationv1.GroupName, Resource: "selfsubjectaccessreviews"},
				"",
				errors.New("forbidden"),
			),
		},
		{
			name:      "unauthorized",
			createErr: apierrors.NewUnauthorized("unauthorized"),
		},
	} {
		t.Run("SSAR_"+tc.name+"_is_non_blocking", func(t *testing.T) {
			recordingClient.verbs = nil
			recordingClient.createErrorVerb = "list"
			recordingClient.createError = tc.createErr

			allowed, reason, err = reconciler.canManageDaemonSets(context.Background(), refreshTestNamespace)
			if err != nil {
				t.Fatalf("expected %s SSAR error to be non-blocking: %v", tc.name, err)
			}
			if allowed || reason == "" {
				t.Fatalf("expected %s SSAR error to disable DaemonSet access, got allowed=%t reason=%q", tc.name, allowed, reason)
			}
		})
	}

	recordingClient.verbs = nil
	recordingClient.createErrorVerb = "list"
	recordingClient.createError = errors.New("temporary API failure")
	allowed, reason, err = reconciler.canManageDaemonSets(context.Background(), refreshTestNamespace)
	if err == nil {
		t.Fatalf("expected temporary SSAR API failure to return an error, got allowed=%t reason=%q", allowed, reason)
	}

	recordingClient.verbs = nil
	recordingClient.createErrorVerb = ""
	recordingClient.createError = nil
	recordingClient.evaluationErrorVerb = "list"
	allowed, reason, err = reconciler.canManageDaemonSets(context.Background(), refreshTestNamespace)
	if err == nil {
		t.Fatalf("expected evaluation error to return an error, got allowed=%t reason=%q", allowed, reason)
	}
}

func newRefreshTestReconciler(t *testing.T, objects []client.Object, permissionChecker func(context.Context, string) (bool, string, error)) (*Reconciler, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add apps/v1 to the test scheme: %v", err)
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

	return &Reconciler{
		ODLMOperator: &deploy.ODLMOperator{
			Client: k8sClient,
			Reader: k8sClient,
		},
		daemonSetPermissionChecker: permissionChecker,
	}, k8sClient
}

func refreshTestWorkloads() []client.Object {
	objectMeta := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{
			Name:      name,
			Namespace: refreshTestNamespace,
			Labels: map[string]string{
				constant.BindInfoRefreshLabel: "enabled",
			},
			Annotations: map[string]string{
				"bindinfoRefresh/" + refreshTestResourceType: refreshTestResourceName,
			},
		}
	}

	return []client.Object{
		&appsv1.Deployment{ObjectMeta: objectMeta("deployment")},
		&appsv1.StatefulSet{ObjectMeta: objectMeta("statefulset")},
		&appsv1.DaemonSet{ObjectMeta: objectMeta("daemonset")},
	}
}

func refreshTestDaemonSet() *appsv1.DaemonSet {
	return refreshTestWorkloads()[2].(*appsv1.DaemonSet)
}

func assertRestarted(t *testing.T, k8sClient client.Client, workload client.Object, name string, expected bool) {
	t.Helper()

	if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: refreshTestNamespace}, workload); err != nil {
		t.Fatalf("failed to get %T %s: %v", workload, name, err)
	}
	_, restarted := workload.GetAnnotations()["bindinfo/restartTime"]
	if restarted != expected {
		t.Fatalf("expected %T %s restarted=%t, got %t", workload, name, expected, restarted)
	}
}
