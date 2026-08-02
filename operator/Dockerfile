# Stage 1 — build the Go binary
FROM golang:1.22 AS builder

WORKDIR /workspace

# Copy go.mod first for layer caching
COPY go.mod go.mod

# Download dependencies
RUN go mod download

# Copy source
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/
COPY hack/ hack/

# Build — statically linked, no CGO, for linux/amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -a -ldflags="-s -w" -o manager ./cmd/

# Stage 2 — minimal runtime image
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
