FROM golang:1.25.12-bookworm@sha256:a9c020ee3d1508c7be5435c262434e3d3fc1d0e76a11afeb9ddae7d60bc86aa4 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify

COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY scenarios/ ./scenarios/

ARG VERSION=0.0.0-ci
ARG COMMIT=none
RUN CGO_ENABLED=0 go build \
    -mod=readonly \
    -trimpath \
    -buildvcs=false \
    -ldflags="-s -w -X=main.version=${VERSION} -X=main.commit=${COMMIT} -X=main.date=container-ci" \
    -o /out/cailab \
    ./cmd/cailab
RUN CGO_ENABLED=0 go build \
    -mod=readonly \
    -trimpath \
    -buildvcs=false \
    -ldflags="-s -w" \
    -o /out/cailab-ci-demo \
    ./internal/tools/cidemo

FROM scratch AS runtime

ARG VERSION=0.0.0-ci
ARG COMMIT=none
LABEL org.opencontainers.image.title="CloudAILab CI demo" \
      org.opencontainers.image.description="Ephemeral clean-environment validation image; not a published CloudAILab distribution" \
      org.opencontainers.image.source="https://github.com/msinclair25/cailab" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}"

COPY --from=builder --chown=65532:65532 --chmod=0555 /out/cailab /usr/local/bin/cailab
COPY --from=builder --chown=65532:65532 --chmod=0555 /out/cailab-ci-demo /usr/local/bin/cailab-ci-demo

USER 65532:65532
WORKDIR /tmp
ENV HOME=/tmp \
    CAILAB_HOME=/tmp/cloudailab-state

ENTRYPOINT ["/usr/local/bin/cailab-ci-demo"]
