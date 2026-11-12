# ── Build stage ──────────────────────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM golang:1.27.4-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /tinyupdater ./cmd/tinyupdater

# ── Runtime stage (distroless) ────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=builder /tinyupdater /tinyupdater

ENTRYPOINT ["/tinyupdater"]

CMD ["server", "--config", "/etc/tinyupdater/config.yaml"]
