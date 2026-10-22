ARG APP_VERSION=dev
ARG VCS_REF=unknown

FROM oven/bun:1.4.0@sha256:5ff609364c049b54eb0ff560ec96319729a972078ef2c755d758f0c6ef89c2d6 AS builder

ARG APP_VERSION

WORKDIR /build/web
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile

ARG VITE_SITE_PROFILE
ARG VITE_SITE_TITLE
ARG VITE_SITE_DESCRIPTION
ARG VITE_SITE_LOGO
ARG VITE_SITE_FAVICON
ARG VITE_SITE_APPLE_TOUCH_ICON
ARG VITE_SITE_BANNER_BRAND
ARG VITE_SITE_DEFAULT_FONT

COPY ./web ./
COPY pkg/billingexpr/calendars/cn.v1.json /build/pkg/billingexpr/calendars/cn.v1.json
RUN DISABLE_ESLINT_PLUGIN='true' \
    VITE_REACT_APP_VERSION="${APP_VERSION}" \
    VITE_SITE_PROFILE="${VITE_SITE_PROFILE}" \
    VITE_SITE_TITLE="${VITE_SITE_TITLE}" \
    VITE_SITE_DESCRIPTION="${VITE_SITE_DESCRIPTION}" \
    VITE_SITE_LOGO="${VITE_SITE_LOGO}" \
    VITE_SITE_FAVICON="${VITE_SITE_FAVICON}" \
    VITE_SITE_APPLE_TOUCH_ICON="${VITE_SITE_APPLE_TOUCH_ICON}" \
    VITE_SITE_BANNER_BRAND="${VITE_SITE_BANNER_BRAND}" \
    VITE_SITE_DEFAULT_FONT="${VITE_SITE_DEFAULT_FONT}" \
    bun run build

FROM golang:1.26.10-alpine@sha256:9ce231b28998b32e116927c61aa5c406183061d4f3f2d61b292c88e7f6bdf7ca AS builder2

ARG APP_VERSION
ENV GO111MODULE=on CGO_ENABLED=0 GOWORK=off

ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}
ENV GOEXPERIMENT=greenteagc

WORKDIR /build

ADD go.mod go.sum ./
# relaykit and tokenkit are local submodules referenced via replace; their
# go.mod files must be present for go mod download to resolve the main module graph.
ADD relaykit/go.mod ./relaykit/go.mod
ADD tokenkit/go.mod ./tokenkit/go.mod
RUN go mod download

COPY . .
COPY --from=builder /build/web/dist ./web/dist
RUN go build -pgo="$(go list -m -f '{{.Dir}}' github.com/Calcium-Ion/moejs)/default.pgo" -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=${APP_VERSION}'" -o new-api \
    && go build -ldflags "-s -w" -o catalog-sync ./cmd/catalog-sync

FROM debian:bookworm-slim@sha256:f06537653ac770703bc45b4b113475bd402f451e85223f0f2837acbf89ab020a

ARG APP_VERSION
ARG VCS_REF

LABEL org.opencontainers.image.title="New API multi-site gateway" \
      org.opencontainers.image.description="Multi-site AI API gateway and administration service" \
      org.opencontainers.image.source="https://github.com/2hot4you/new-api" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.version="${APP_VERSION}" \
      org.opencontainers.image.licenses="AGPL-3.0-only"

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata libasan8 wget \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

COPY --from=builder2 /build/new-api /
COPY --from=builder2 /build/catalog-sync /
COPY LICENSE NOTICE THIRD-PARTY-LICENSES.md /licenses/
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/new-api"]
