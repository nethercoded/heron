# syntax=docker/dockerfile:1.12

FROM node:26-alpine AS web
WORKDIR /src/web
RUN npm install -g pnpm@12.16.1
COPY web/package.json web/pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=web /src/web/build ./web/build
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/heron ./cmd/heron
RUN mkdir -p /out/data

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/heron /usr/local/bin/heron
COPY --from=build --chown=65532:65532 /out/data /data
ENV HERON_LISTEN=:8080 \
    HERON_DATA_DIR=/data \
    HERON_LOG_FORMAT=json
VOLUME /data
EXPOSE 8080 80 443
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/heron"]
CMD ["serve"]
