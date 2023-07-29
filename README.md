# kubelogin

[![Go Report Card](https://goreportcard.com/badge/github.com/Azure/kubelogin)](https://goreportcard.com/report/github.com/Azure/kubelogin)
[![golangci-lint](https://github.com/Azure/kubelogin/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/Azure/kubelogin/actions/workflows/golangci-lint.yml)
[![Build on Push](https://github.com/Azure/kubelogin/actions/workflows/build.yml/badge.svg)](https://github.com/Azure/kubelogin/actions/workflows/build.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Azure/kubelogin.svg)](https://pkg.go.dev/github.com/Azure/kubelogin)
[![codecov](https://codecov.io/gh/Azure/kubelogin/branch/master/graph/badge.svg?token=02PZRX59VM)](https://codecov.io/gh/Azure/kubelogin)

This is a [client-go credential (exec) plugin](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins) implementing azure authentication. This plugin provides features that are not available in kubectl. It is supported on kubectl v1.11+

Check out [the official doc page](https://azure.github.io/kubelogin/index.html) for more details

## Features in this fork

For avoiding having to log out and redo device login to pick up JIT groups:

  - set the environment variable `KUBELOGIN_FORCE_REFRESH=1` to make
    `kubelogin` use its refresh token to get a new access token, instead of
    using the cached one that may not have the JIT groups.

  - set the environment variable `KUBELOGIN_VERBOSE=10` to enable debug
    logging

To install:

  1. Check out this repo
  2. Run `go install`
  3. Make sure `~/go/bin` comes earlier in your `$PATH` than wherever the
     pre-existing `kubelogin` binary is

## Installation

https://azure.github.io/kubelogin/install.html

## Quick Start

https://azure.github.io/kubelogin/quick-start.html

## Contributing

This project welcomes contributions and suggestions. Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit <https://cla.opensource.microsoft.com>.

When you submit a pull request, a CLA bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., status check, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.
