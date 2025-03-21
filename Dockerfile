FROM --platform=$BUILDPLATFORM public.ecr.aws/docker/library/golang:1-alpine as builder

ARG GOPROXY="direct"

# Copy go.mod and download dependencies
WORKDIR /app
ARG TARGETOS TARGETARCH
ARG GOOS=$TARGETOS
ARG GOARCH=$TARGETARCH
ARG CGO_ENABLED=0

COPY go.mod .
COPY go.sum .
RUN apk update && apk add git
RUN go mod download

COPY . .
RUN go build -o /bin/node-latency-for-k8s cmd/node-latency-for-k8s/main.go

FROM public.ecr.aws/amazonlinux/amazonlinux:2023-minimal as journalctl
WORKDIR /
RUN dnf update -y && dnf install -y systemd && dnf clean all

FROM public.ecr.aws/amazonlinux/amazonlinux:2023-minimal 
WORKDIR /
COPY --from=builder /bin/node-latency-for-k8s .
COPY --from=builder /app/THIRD_PARTY_LICENSES .
WORKDIR /usr/bin
COPY --from=journalctl /usr/bin/journalctl .
WORKDIR /
USER 1000
ENTRYPOINT ["/node-latency-for-k8s"]
