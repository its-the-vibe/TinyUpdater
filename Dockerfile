FROM golang:alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /tinyupdater ./cmd/tinyupdater

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /tinyupdater /tinyupdater
ENTRYPOINT ["/tinyupdater"]
CMD ["server", "--config", "/etc/tinyupdater/config.yaml"]
