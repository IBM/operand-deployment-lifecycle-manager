#!/bin/bash
#
# USER ACTION REQUIRED:
#   This is a scaffold file intended for the user to modify with their own CASE information.
#   Please delete this comment after the proper modifications have been done.
#
# Test script REQUIRED to test your operator
#
set -o errexit
set -o nounset
set -o pipefail

testDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
