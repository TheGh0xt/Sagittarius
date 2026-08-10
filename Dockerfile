# syntax=docker/dockerfile:1

# ---- build stage -------------------------------------------------------
FROM golang:1.26 AS build

WORKDIR /src

# Dependency layer: cached until go.mod/go.sum actually change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO off + static linking so the binary runs on a distroless base.
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w" -o /out/sagittarius-mcp ./cmd/sagittarius-mcp

# ---- runtime stage -----------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /
COPY --from=build /out/sagittarius-mcp /sagittarius-mcp

# Layer 1 is stateless: no volumes, no writable paths needed.
USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/sagittarius-mcp"]
CMD ["--transport=http", "--port=8080"]
