# The demo image: the memory layer (replaying the frozen story), the
# ledger single-page app, the public edge, the first-boot seeder, the
# seeded population fixture behind the pool, and the Lambo 0.10.1 release
# binary. Only the data dir lives on the persistent disk
# (/var/lambo-data); everything else ships in the
# image, because the disk is visible to the running instance alone.
#
# Deviation from research/hosting.md, recorded: runtime is docker, not
# go. The frozen service cannot serve the SPA or the public webhook
# path, and one image keeps the settled single-service shape. Plan,
# disk, region and cost are unchanged (see render.yaml).

# --- Go: the service, the edge, the seeder (CGO-free, static) ---
FROM golang:1.27-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY seed/ ./seed/
COPY deploy/edge/ ./deploy/edge/
COPY deploy/seed/ ./deploy/seed/
RUN CGO_ENABLED=0 go build -trimpath -o /out/memento ./cmd/memento && \
	CGO_ENABLED=0 go build -trimpath -o /out/edge ./deploy/edge && \
	CGO_ENABLED=0 go build -trimpath -o /out/seed ./deploy/seed

# --- Web: the ledger app (static build) ---
FROM node:22-bookworm AS web-build
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
# The ledger reads its fixtures straight from the frozen contract
# examples, so the build stage carries testdata/api beside web/.
COPY testdata/api /src/testdata/api
RUN npm run build

# --- Lambo 0.10.1 release binary (sha-pinned) ---
FROM debian:bookworm-slim AS lambo-fetch
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL -o /tmp/lambo \
		https://github.com/nrynss/lambo/releases/download/v0.10.1/lambo-0.10.1-linux-x86_64 && \
	curl -fsSL -o /tmp/lambo.sha256 \
		https://github.com/nrynss/lambo/releases/download/v0.10.1/lambo-0.10.1-linux-x86_64.sha256 && \
	echo "$(cut -d' ' -f1 /tmp/lambo.sha256)  /tmp/lambo" | sha256sum -c - && \
	chmod +x /tmp/lambo && /tmp/lambo --version

# --- Runtime (trixie: the release binary needs glibc 2.39+) ---
FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=go-build /out/memento /out/edge /out/seed /app/bin/
COPY --from=lambo-fetch /tmp/lambo /app/bin/lambo
COPY --from=web-build /src/web/build /app/web
COPY testdata/api /app/testdata/api
COPY testdata/jev /app/testdata/jev
COPY testdata/seed/population /app/testdata/seed/population
COPY deploy/start.sh /app/start.sh
RUN chmod +x /app/start.sh && /app/bin/lambo --version
ENV LAMBO_DATA_DIR=/var/lambo-data
CMD ["/app/start.sh"]
