<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Contributing guidelines](#contributing-guidelines)
    - [Developer Certificate of Origin](#developer-certificate-of-origin)
    - [Contributing A Patch](#contributing-a-patch)
    - [Issue and Pull Request Management](#issue-and-pull-request-management)
    - [Pre-check before submitting a PR](#pre-check-before-submitting-a-pr)
    - [Build images](#build-images)

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

## Pre-check before submitting a PR

After your PR is ready to commit, please run following commands to check your code.

```shell
make check
make test
```

## Build images

Make sure your code build passed.

```shell
export BUILD_LOCALLY=1
make
```

Now, you can follow the [getting started guide](./README.md#getting-started) to work with the xxx.
