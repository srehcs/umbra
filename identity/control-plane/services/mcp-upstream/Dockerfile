FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download || true
COPY services/mcp-upstream ./services/mcp-upstream
RUN cd services/mcp-upstream && CGO_ENABLED=0 go build -o /out/mcp-upstream ./cmd/server

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/mcp-upstream /mcp-upstream
USER nonroot:nonroot
EXPOSE 8090
ENTRYPOINT ["/mcp-upstream"]
