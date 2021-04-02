module github.com/IBM/operand-deployment-lifecycle-manager

go 1.15

require (
	github.com/IBM/controller-filtered-cache v0.2.0
	github.com/IBM/ibm-namespace-scope-operator v1.0.0-alpha
	github.com/coreos/etcd-operator v0.9.4
	github.com/deckarep/golang-set v1.7.1
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/operator-framework/api v0.3.10
	github.com/pkg/errors v0.9.1
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	k8s.io/klog v1.0.0
	sigs.k8s.io/controller-runtime v0.6.4
	sigs.k8s.io/kubebuilder v1.0.9-0.20200805184228-f7a3b65dd250
)

// fix vulnerability: CVE-2021-3121 in github.com/gogo/protobuf v1.2.1
replace github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
