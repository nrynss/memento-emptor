# The merchant service image: the merchant single-page app plus a tiny
# edge that serves it and proxies the layer's product_fit read with the
# merchant key. Stateless, so no disk: plan starter (0.5 CPU, 512 MB RAM),
# region virginia, $7.00/mo (see render.yaml).

# --- Go: the merchant edge (CGO-free, static) ---
FROM golang:1.27-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY deploy/merchant/ ./deploy/merchant/
RUN CGO_ENABLED=0 go build -trimpath -o /out/merchant-edge ./deploy/merchant

# --- Web: the merchant app (static build) ---
# The pool fixtures live in testdata/api at the repo root (the app's
# fixture transport imports the frozen product_fit example), so the root
# testdata tree is part of this stage's context, like the main image.
FROM node:22-bookworm AS web-build
WORKDIR /src/merchant
COPY merchant/package.json merchant/package-lock.json ./
RUN npm ci
COPY merchant/ ./
COPY testdata/api /src/testdata/api
RUN npm run build

# --- Runtime (trixie: matches the main demo image) ---
FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=go-build /out/merchant-edge /app/merchant-edge
COPY --from=web-build /src/merchant/build /app/merchant
ENV MERCHANT_STATIC=/app/merchant
CMD ["/app/merchant-edge"]
