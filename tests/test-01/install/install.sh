#!/bin/bash
#
# USER ACTION REQUIRED:
#   This is a scaffold file intended for the user to modify with their own CASE information.
#   Please delete this comment after the proper modifications have been done.
#
# Install script to install the operator
#
set -o errexit
set -o nounset
set -o pipefail

installDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

$APP_TEST_LIBRARY_FUNCTIONS/operatorInstall.sh \
	--cr $CV_TEST_BUNDLE_DIR/operators/meta-operator/deploy/crds/FIXME_cr.yaml
