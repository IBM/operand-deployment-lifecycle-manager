<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Contributing guidelines](#contributing-guidelines)
  - [Developer Certificate of Origin](#developer-certificate-of-origin)
  - [Contributing A Patch](#contributing-a-patch)
  - [Issue and Pull Request Management](#issue-and-pull-request-management)
  - [Contribution flow](#contribution-flow)
  - [Pre-check before submitting a PR](#pre-check-before-submitting-a-pr)
  - [Build Operator Image](#build-operator-image)
  - [Build Bundle Image](#build-bundle-image)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Contributing guidelines

## Developer Certificate of Origin

This repository built with [probot](https://github.com/probot/probot) that enforces the [Developer Certificate of Origin](https://developercertificate.org/) (DCO) on Pull Requests. It requires all commit messages to contain the `Signed-off-by` line with an email address that matches the commit author.

## Contributing A Patch

1. Submit an issue describing your proposed change to the repo in question.
1. The [repo owners](OWNERS) will respond to your issue promptly.
1. Fork the desired repo, develop and test your code changes.
1. Commit your changes with DCO
1. Submit a pull request.

## Issue and Pull Request Management

Anyone may comment on issues and submit reviews for pull requests. However, in
order to be assigned an issue or pull request, you must be a member of the
[IBM](https://github.com/ibm) GitHub organization.

Repo maintainers can assign you an issue or pull request by leaving a
`/assign <your Github ID>` comment on the issue or pull request.

## Contribution flow

This is a rough outline of what a contributor's workflow looks like:

- Create a topic branch from where to base the contribution. This is usually master.
- Make commits of logical units.
- Make sure commit messages are in the proper format (see below).
- Push changes in a topic branch to a personal fork of the repository.
- Submit a pull request to IBM/operator-deployment-lifecycle-manager.
- The PR must receive a LGTM from two maintainers found in the MAINTAINERS file.

Thanks for contributing!

## Pre-check before submitting a PR

After your PR is ready to commit, please run following commands to check your code and run the unit test.

```shell
make code-dev
```

Then you need to make sure it can pass the e2e test

```shell
make e2e-test-kind
```

## Build Operator Image

Make sure your code build passed.

```shell
make build-operator-image
```

## Build Bundle Image

You can use the following command to build the operator bundle image

```shell
make build-bundle-image
```
