package k8sutil

import (
	"io"
	"net/http"
	"strings"
	"testing"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

type discoveryTransport struct {
	calls int
	olm   bool
}

func (d *discoveryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	d.calls++
	body := `{"kind":"APIVersions","apiVersion":"v1","versions":["v1"]}`
	if req.URL.Path == "/apis" {
		body = `{"kind":"APIGroupList","apiVersion":"v1","groups":[]}`
		if d.olm {
			body = `{"kind":"APIGroupList","apiVersion":"v1","groups":[{"name":"operators.coreos.com","versions":[{"groupVersion":"operators.coreos.com/v1alpha1","version":"v1alpha1"}],"preferredVersion":{"groupVersion":"operators.coreos.com/v1alpha1","version":"v1alpha1"}}]}`
		}
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func TestNewODLMCacheInstallMode(t *testing.T) {
	for _, tc := range []struct {
		name       string
		noOLM      string
		clusterOLM bool
		wantOLM    bool
	}{
		{"Helm on OLM cluster", "true", true, false},
		{"Helm without OLM", "true", false, false},
		{"OLM installation", "false", true, true},
		{"OLM API absent", "false", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NO_OLM", tc.noOLM)
			t.Setenv("WATCH_NAMESPACE", "test")
			transport := &discoveryTransport{olm: tc.clusterOLM}
			opts := NewODLMCache(true, ctrl.Options{}, &rest.Config{Host: "https://cluster.example", Transport: transport})
			csv, sub := false, false
			for obj := range opts.Cache.ByObject {
				switch obj.(type) {
				case *olmv1alpha1.ClusterServiceVersion:
					csv = true
				case *olmv1alpha1.Subscription:
					sub = true
				}
			}
			if csv != tc.wantOLM || sub != tc.wantOLM {
				t.Fatalf("CSV cached=%v, Subscription cached=%v; want both %v", csv, sub, tc.wantOLM)
			}
			if tc.noOLM == "true" && transport.calls != 0 {
				t.Fatalf("Helm mode made %d discovery requests", transport.calls)
			}
			if tc.noOLM != "true" && transport.calls == 0 {
				t.Fatal("OLM mode did not check API availability")
			}
		})
	}
}
