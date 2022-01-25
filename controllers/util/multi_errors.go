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

package util

import (
	"strings"
)

// MultiErr is a multiple error slice
type MultiErr struct {
	Errors []string
}

// Error is the error message
func (mer *MultiErr) Error() string {
	if len(mer.Errors) == 0 {
		return "no error occurred"
	}
	var sb strings.Builder
	sb.WriteString("the following errors occurred:")
	for _, errMessage := range mer.Errors {
		sb.WriteString("\n  - " + errMessage)
	}
	return sb.String()
}

// Add appends error message
func (mer *MultiErr) Add(err error) {
	if mer.Errors == nil {
		mer.Errors = []string{}
	}
	mer.Errors = append(mer.Errors, err.Error())
}
