FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY apps/api-go/ ./
RUN go mod download && CGO_ENABLED=0 go build -mod=readonly -trimpath -o /api ./cmd/api
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
