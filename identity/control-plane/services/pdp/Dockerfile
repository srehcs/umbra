FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download || true
COPY packages ./packages
COPY services/pdp ./services/pdp
WORKDIR /src/services/pdp
RUN CGO_ENABLED=0 go build -o /out/pdp ./cmd/server

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/pdp /pdp
USER nonroot:nonroot
EXPOSE 8081
ENTRYPOINT ["/pdp"]
