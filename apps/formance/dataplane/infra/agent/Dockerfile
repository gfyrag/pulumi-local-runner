FROM golang:1.25-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -o /bin/agent \
    -ldflags "-extldflags '-static'" \
    .

FROM ghcr.io/formancehq/base:22.04
COPY --from=builder /bin/agent /usr/bin/agent
ENV OTEL_SERVICE_NAME=agent
ENTRYPOINT ["/usr/bin/agent"]
