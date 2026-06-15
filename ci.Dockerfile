# build web
FROM --platform=$BUILDPLATFORM node:24.12.0-alpine AS web-builder
RUN npm install --global corepack@latest
RUN corepack enable

WORKDIR /web

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY web ./
RUN pnpm run build

# build app
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine AS app-builder

ARG VERSION=dev
ARG REVISION=dev
ARG BUILDTIME
ARG TARGETOS TARGETARCH

RUN apk add --no-cache git build-base tzdata

ENV SERVICE=shinkro

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
COPY --from=web-builder /web/dist ./web/dist
COPY --from=web-builder /web/build.go ./web

RUN GOOS="$TARGETOS" GOARCH="$TARGETARCH" go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${REVISION} -X main.date=${BUILDTIME}" -o /out/bin/shinkro cmd/shinkro/main.go

# build runner
FROM alpine:latest

LABEL org.opencontainers.image.source="https://github.com/varoOP/shinkro"

ENV HOME="/config" \
    XDG_CONFIG_HOME="/config" \
    XDG_DATA_HOME="/config"

RUN apk --no-cache add ca-certificates curl tzdata jq

WORKDIR /app

VOLUME /config

COPY --from=app-builder /out/bin/shinkro /usr/local/bin/

EXPOSE 7011

ENTRYPOINT ["/usr/local/bin/shinkro", "--config", "/config"]