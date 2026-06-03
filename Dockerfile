# Self-contained multi-stage build. Usable with a plain `docker build .`.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=docker
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w -X github.com/debipro/cli/pkg/version.Version=${VERSION}" \
    -o /debi ./cmd/debi

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /debi /usr/local/bin/debi
ENTRYPOINT ["/usr/local/bin/debi"]
