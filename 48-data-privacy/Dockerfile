# Build stage
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/vault ./cmd/vault

# Runtime stage: distroless, non-root, static binary.
FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/vault /vault
# Configuration is supplied via environment variables (see README).
# VAULT_MASTER_KEY and API_KEYS / API_KEYS_FILE are required at runtime.
EXPOSE 8443
ENTRYPOINT ["/vault"]
CMD ["serve"]
