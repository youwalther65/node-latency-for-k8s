#!/usr/bin/env bash
set -euo pipefail

go install github.com/google/go-licenses@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/google/ko@latest
go install github.com/norwoodj/helm-docs/cmd/helm-docs@latest
go install github.com/sigstore/cosign/cmd/cosign@latest
go install golang.org/x/vuln/cmd/govulncheck@latest

if [[ $(go env GOOS) = "darwin" ]]; then
    brew install podman
else
    apt-get -y install podman
fi