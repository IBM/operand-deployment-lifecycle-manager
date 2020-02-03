<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Running E2E Tests](#running-e2e-tests)
    - [Reference](#reference)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Running E2E Tests

1. Ensure **operator-sdk** is installed.

1. If the namespace **common-service-operator** exists, delete it. Do this to make sure the namespace is clean.

    ```bash
    oc delete namespace common-service-operator
    ```

1. Create the namespace **common-service-operator**.

    ```bash
    oc create namespace common-service-operator
    ```

1. Run the test using `make test-e2e`  command locally.

    ```bash
    make test-e2e
    ```

## Reference

- [Running tests](https://github.com/operator-framework/operator-sdk/blob/master/doc/test-framework/writing-e2e-tests.md#running-the-tests)
- [Installing Operator-SDK](https://github.com/operator-framework/operator-sdk#quick-start)
