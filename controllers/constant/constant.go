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

package constant

import (
	"time"
)

const (

	//ClusterOperatorNamespace is the namespace of cluster operators
	ClusterOperatorNamespace string = "openshift-operators"

	//NotUninstallLabel is the label used to prevent subscription/CR from uninstall
	NotUninstallLabel string = "operator.ibm.com/opreq-do-not-uninstall"

	//OpreqLabel is the label used to label the Subscription/CR/Configmap managed by ODLM
	OpreqLabel string = "operator.ibm.com/opreq-control"

	//OpreqTrackerLabel is the label used to label the OperandRequest Configmap managed by ODLM
	OpreqTrackerLabel string = "operator.ibm.com/operand-request-tracker"

	//InternalOpreqLabel is the label used label the OperandRequest internally created by ODLM
	OperandOnlyLabel string = "operator.ibm.com/operand-only"

	//ODLMReferenceAnnotation is the annotation used to annotate the resources used for ODLM operand value reference
	ODLMReferenceAnnotation string = "operator.ibm.com/referenced-by-odlm-resource"

	//ODLMWatchedLabel is the label used to label the resources watched by ODLM for value reference
	ODLMWatchedLabel string = "operator.ibm.com/watched-by-odlm"

	//OpbiNsLabel is the label used to add OperandBindInfo namespace to the secrets/configmaps watched by ODLM
	OpbiNsLabel string = "operator.ibm.com/watched-by-opbi-with-namespace"

	//OpbiNameLabel is the label used to add OperandBindInfo name to the secrets/configmaps watched by ODLM
	OpbiNameLabel string = "operator.ibm.com/watched-by-opbi-with-name"

	//OpbiTypeLabel is the label used to label if secrets/configmaps are "original" or "copy"
	OpbiTypeLabel string = "operator.ibm.com/managedBy-opbi"

	//BindInfoRefreshLabel is the label used to label if secrets/configmaps are "original" or "copy"
	BindInfoRefreshLabel string = "operator.ibm.com/bindinfoRefresh"

	//NamespaceScopeCrName is the name use to get NamespaceScopeCrName instance
	NamespaceScopeCrName string = "nss-managedby-odlm"

	//OdlmScopeNssCrName is the name use to get OdlmScopeNssCrName instance
	OdlmScopeNssCrName string = "odlm-scope-managedby-odlm"

	//FindOperandRegistry is the key for checking if the OperandRegistry is found
	FindOperandRegistry string = "operator.ibm.com/operandregistry-is-not-found"

	//HashedData is the key for checking the checksum of data section
	HashedData string = "hashedData"

	//HashedData is the key for k8s Resource
	K8sHashedData string = "operator.ibm.com/operand-depoyment-lifecycle-manager.hashedData"

	//RouteHash is the key for hash value of route
	RouteHash string = "operator.ibm.com/odlm.route.hashedData"

	//DefaultRequestTimeout is the default timeout for kube request
	DefaultRequestTimeout = 5 * time.Second

	//DefaultRequeueDuration is the default requeue time duration for request
	DefaultRequeueDuration = 20 * time.Second

	//DefaultSyncPeriod is the frequency at which watched resources are reconciled
	DefaultSyncPeriod = 3 * time.Hour

	//DefaultCRFetchTimeout is the default timeout for getting a custom resource
	DefaultCRFetchTimeout = 250 * time.Millisecond

	//DefaultCRFetchPeriod is the default retry Period for getting a custom resource
	DefaultCRFetchPeriod = 5 * time.Second

	//DefaultCRDeleteTimeout is the default timeout for deleting a custom resource
	DefaultCRDeleteTimeout = 5 * time.Minute

	//DefaultCRDeletePeriod is the default retry Period for deleting a custom resource
	DefaultCRDeletePeriod = 20 * time.Second

	//DefaultSubDeleteTimeout is the default timeout for deleting a subscription
	DefaultSubDeleteTimeout = 10 * time.Minute

	//DefaultCSVWaitPeriod is the default period for wait CSV ready
	DefaultCSVWaitPeriod = 1 * time.Minute

	//DefaultCRRetryNumber is the default maximum number of retry for reconciling a custom resource
	DefaultCRRetryNumber = 3

	//StatusMonitoredServices is the annotation key for monitored services
	StatusMonitoredServices = "status-monitored-services"
)
