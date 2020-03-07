#
# Copyright 2020 IBM Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

FROM registry.access.redhat.com/ubi8/ubi-minimal@sha256:47e2f82dbede8ff990e6e240f82d78830e7558f7b30df7bd8c0693992018b1e3
ARG VCS_REF
ARG VCS_URL

LABEL org.label-schema.vendor="IBM" \
  org.label-schema.name="odlm" \
  org.label-schema.description="Manager the lifecycle of the operands" \
  org.label-schema.vcs-ref=$VCS_REF \
  org.label-schema.vcs-url=$VCS_URL \
  org.label-schema.license="Licensed Materials - Property of IBM" \
  org.label-schema.schema-version="1.0" \
  name="odlm" \
  vendor="IBM" \
  description="Manager the lifecycle of the operands" \
  summary="Manager the lifecycle of the operands"

ENV OPERATOR=/usr/local/bin/odlm \
  DEPLOY_DIR=/deploy \
  USER_UID=1001 \
  USER_NAME=odlm

# install operator binary
COPY build/_output/bin/odlm ${OPERATOR}
COPY deploy/crds ${DEPLOY_DIR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

# copy licenses
RUN mkdir /licenses
COPY LICENSE /licenses

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
