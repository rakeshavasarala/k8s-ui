# Build stage
FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the binary.
# CGO_ENABLED=0 is important for static linking.
RUN CGO_ENABLED=0 GOOS=linux go build -o k8s-ui ./cmd/server

# Run stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /app/k8s-ui /k8s-ui

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/k8s-ui"]
