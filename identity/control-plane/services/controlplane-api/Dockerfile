FROM golang:1.22-alpine AS build
WORKDIR /src
COPY ../../go.mod ../../go.sum ./
RUN go mod download || true
COPY ../../packages ./packages
COPY . .
RUN CGO_ENABLED=0 go build -o /out/controlplane-api ./cmd/server

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/controlplane-api /controlplane-api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/controlplane-api"]
