# Stage 1 — build the Go binary
FROM golang:1.23 AS builder

WORKDIR /workspace

# Copy everything first so 'go mod tidy' can see all imports
COPY go.mod go.mod
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/
COPY hack/ hack/

# Tidy resolves all imports and writes go.sum, then download fetches them
RUN go mod tidy && go mod download

# Build — statically linked, no CGO
RUN CGO_ENABLED=0 GOOS=linux \
    go build -a -ldflags="-s -w" -o manager ./cmd/

# Stage 2 — minimal runtime image
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
