FROM golang:1.25-bookworm AS builder
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/portal-public-config ./cmd/portal-public-config

FROM gcr.io/distroless/static-debian12:nonroot
USER nonroot:nonroot
COPY --from=builder /out/portal-public-config /portal-public-config

ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/portal-public-config"]
