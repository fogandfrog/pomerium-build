# syntax=docker/dockerfile:1
# The workflow supplies a verified digest. Without it, the required Envoy COPY
# fails against scratch rather than downloading an unreviewed fallback image.
ARG ENVOY_IMAGE=scratch
FROM ${ENVOY_IMAGE} AS envoy
# Source is checked out separately at the resolved upstream commit. No official
# Pomerium Core binary/image is used. Retain upstream build metadata for Trivy.
FROM node:24.22.2-bookworm@sha256:8fa96264f302f58bbb13aa7339c541547b56975766ce956c875da149452a3e40 AS ui
WORKDIR /build/ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci --ignore-scripts && npm audit --audit-level=critical
COPY ui/ ./
RUN --network=none npm run build
RUN mkdir -p /notices && find node_modules -type f \
    \( -iname '*license*' -o -iname '*licence*' -o -iname '*notice*' -o -iname 'copying*' -o -iname 'copyright*' -o -iname 'ofl*' -o -name package.json \) \
    -exec cp --parents '{}' /notices/ \;

FROM golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea AS build
WORKDIR /build
RUN apt-get update && apt-get upgrade -y --with-new-pkgs \
    && apt-get install -y --no-install-recommends zip && rm -rf /var/lib/apt/lists/*
COPY . .
COPY --from=ui /build/ui/dist ./ui/dist
RUN go mod download && go mod verify
COPY --from=envoy /envoy ./pkg/envoy/files/envoy-linux-amd64
COPY --from=envoy /licenses ./build-notices/envoy
ARG ENVOY_COMMIT
ARG ENVOY_TIMESTAMP
# Use upstream's typed lockfile and version calculation. Never download an
# official Envoy binary, replace a dependency, or alter a license stamp.
RUN --network=none go run -mod=readonly ./independent-build/embed-envoy \
    pkg/envoy/files/envoy-linux-amd64 "$ENVOY_COMMIT" "$ENVOY_TIMESTAMP"
RUN --network=none go test ./internal/version ./internal/analytics ./pkg/envoy/files
ARG POMERIUM_VERSION
ARG POMERIUM_COMMIT
RUN --network=none CGO_ENABLED=0 go build -mod=readonly \
    -ldflags="-s -w -X github.com/pomerium/pomerium/internal/version.Version=${POMERIUM_VERSION} -X github.com/pomerium/pomerium/internal/version.GitCommit=${POMERIUM_COMMIT} -X github.com/pomerium/pomerium/internal/version.officialBuild=0" \
    -o /out/pomerium ./cmd/pomerium
RUN touch /out/config.yaml
COPY --from=ui /notices /out/notices/npm
RUN mkdir -p /out/notices/go /out/notices/runtime \
    && cp LICENSE /out/notices/POMERIUM-LICENSE \
    && cp -a build-notices/. /out/notices/ \
    && cp /usr/local/go/LICENSE /out/notices/runtime/GO-LICENSE \
    && cd /go/pkg/mod && find . -type f \
    \( -iname '*license*' -o -iname '*licence*' -o -iname '*notice*' -o -iname 'copying*' -o -iname 'copyright*' -o -name AUTHORS -o -name PATENTS \) \
    -exec cp --parents '{}' /out/notices/go/ \;

FROM gcr.io/distroless/base-nossl-debian13:latest@sha256:ca1c7e6a875e59345ae6ff182a1e0930f0ca1ffef23a9b830f8823ab348ad5dd
ENV AUTOCERT_DIR=/data/autocert \
    OTEL_TRACES_EXPORTER=none \
    DO_NOT_TRACK=1 \
    RUNTIME_FLAGS='{"product_analytics":false,"routes_portal":false}'
WORKDIR /pomerium
COPY --from=build /out/pomerium /bin/pomerium
COPY --from=build /out/config.yaml /pomerium/config.yaml
COPY --from=build /build/LICENSE /pomerium/LICENSE
COPY --from=build /out/notices /usr/share/licenses/pomerium-build
ENTRYPOINT ["/bin/pomerium"]
CMD ["--config", "/pomerium/config.yaml"]
