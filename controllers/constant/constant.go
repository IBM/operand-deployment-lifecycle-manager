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

package constant

const (

	//ClusterOperatorNamespace is the namespace of cluster operators
	ClusterOperatorNamespace string = "openshift-operators"

	//NotUninstallLabel is the label used to prevent subscription/CR from uninstall
	NotUninstallLabel string = "operator.ibm.com/opreq-do-not-uninstall"

	//OpreqLabel is the label used to label the subscription/CR managed by ODLM
	OpreqLabel string = "operator.ibm.com/opreq-control"

	//OpbiNsLabel is the label used to add OperandBindInfo namespace to the secrets/configmaps watched by ODLM
	OpbiNsLabel string = "operator.ibm.com/watched-by-opbi-with-namespace"

	//OpbiNameLabel is the label used to add OperandBindInfo name to the secrets/configmaps watched by ODLM
	OpbiNameLabel string = "operator.ibm.com/watched-by-opbi-with-name"

	//OpbiTypeLabel is the label used to label if secrets/configmaps are "original" or "copy"
	OpbiTypeLabel string = "operator.ibm.com/managedBy-opbi"
)
