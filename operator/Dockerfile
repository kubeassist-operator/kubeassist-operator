# Stage 1 — build the Go binary
FROM golang:1.23 AS builder

WORKDIR /workspace

# Copy go.mod first for layer caching
COPY go.mod go.mod

# Tidy and download — regenerates go.sum inside the build container
# so the host go.sum does not need to be kept in sync manually.
RUN go mod tidy && go mod download

# Copy source
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/
COPY hack/ hack/

# Build — statically linked, no CGO
RUN CGO_ENABLED=0 GOOS=linux \
    go build -a -ldflags="-s -w" -o manager ./cmd/

# Stage 2 — minimal runtime image
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
