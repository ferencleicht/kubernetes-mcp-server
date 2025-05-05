ARG VERSION="dev"

FROM golang:1.24.2 AS build
ARG VERSION
WORKDIR /build

RUN go env -w GOMODCACHE=/root/.cache/go-build

# Install dependencies
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . ./

# Build the application
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    go build -ldflags="-s -w -X main.version=${VERSION} -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o kubernetes-mcp-server cmd/kubernetes-mcp-server/main.go

FROM gcr.io/distroless/base-debian12

WORKDIR /app
COPY --from=build /build/kubernetes-mcp-server .
CMD [ "./kubernetes-mcp-server", "stdio" ]
