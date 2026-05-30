FROM golang:1.25-bookworm AS builder
ENV GOPRIVATE=github.com/formancehq/*
WORKDIR /src
COPY go.mod go.sum ./
COPY pkg/client/go.mod pkg/client/go.sum ./pkg/client/
RUN go mod download
COPY main.go ./
COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/
COPY web/ web/
RUN CGO_ENABLED=0 go build -o /bin/membership \
    -ldflags "-extldflags '-static'" \
    .

FROM ghcr.io/formancehq/base:22.04
COPY --from=builder /bin/membership /usr/bin/membership
ENV OTEL_SERVICE_NAME=membership
ENTRYPOINT ["/usr/bin/membership"]
CMD ["serve"]
