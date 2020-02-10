#!/bin/bash
#
# USER ACTION REQUIRED:
#   This is a scaffold file intended for the user to modify with their own CASE information.
#   Please delete this comment after the proper modifications have been done.
#
# Delete script for resources tested
#
set -o errexit
set -o nounset
set -o pipefail

deleteDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

$APP_TEST_LIBRARY_FUNCTIONS/operatorDelete.sh \
    --serviceaccount $CV_TEST_BUNDLE_DIR/operators/meta-operator/deploy/service_account.yaml \
    --crd $CV_TEST_BUNDLE_DIR/operators/meta-operator/deploy/crds/FIXME_crd.yaml \
    --cr $CV_TEST_BUNDLE_DIR/operators/meta-operator/deploy/crds/FIXME_cr.yaml \
    --role $CV_TEST_BUNDLE_DIR/operators/meta-operator/deploy/role.yaml \
    --rolebinding $CV_TEST_BUNDLE_DIR/operators/meta-operator/deploy/role_binding.yaml \
    --operator $CV_TEST_BUNDLE_DIR/operators/meta-operator/deploy/operator.yaml
