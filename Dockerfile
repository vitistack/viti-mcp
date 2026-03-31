FROM golang:1.26 AS builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o viti-mcp ./cmd/viti-mcp

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /workspace/viti-mcp /usr/local/bin/viti-mcp

USER 65534:65534

ENTRYPOINT ["viti-mcp"]
