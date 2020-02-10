#!/bin/bash
#
# USER ACTION REQUIRED:
#   This is a scaffold file intended for the user to modify with their own CASE information.
#   Please delete this comment after the proper modifications have been done.
#
# Wait script for resouces to become available in the cluster
#
set -o errexit
set -o nounset
set -o pipefail

waitDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

echo "Start wait.sh file...."

  dep="meta-operator"
  retries=20 # 10 minute timeout

  while ! kubectl rollout status -w "deployment/${dep}" --namespace=$CV_TEST_NAMESPACE; do
      sleep 30
      retries=$((retries - 1))
      if [[ $retries == 0 ]]; then
        echo "FAIL: Failed to rollout deployloyment $dep"
        exit 1
      fi
      echo "retrying check rollout status for deployment $dep..."
  done
  # kubectl get deployments -l app.kubernetes.io/instance=$labelname -n $namespace
echo "Successfully rolled out deployment \"$dep\" in namespace \"$CV_TEST_NAMESPACE\""
