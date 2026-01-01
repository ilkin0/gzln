# syntax=docker/dockerfile:1
ARG GO_VERSION=1.25.5

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build

WORKDIR /src

# Copy go.mod and go.sum for dependency caching
COPY go.mod go.sum ./

# Cache dependencies
RUN --mount=type=cache,target=/go/pkg/mod \
  go mod download -x

# Copy the entire source code
COPY . .

ARG TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod \
  CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /bin/server ./cmd/server

FROM alpine:3.20 AS final

RUN apk add --no-cache ca-certificates tzdata && update-ca-certificates

ARG UID=10001

RUN adduser \
  --disabled-password \
  --gecos "" \
  --home "/nonexistent" \
  --shell "/sbin/nologin" \
  --uid "${UID}" appuser

USER appuser

COPY --from=build /bin/server /bin/

EXPOSE 8080

ENTRYPOINT ["/bin/server"]
