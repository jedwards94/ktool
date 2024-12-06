# Stage 1: Build the Go application
FROM golang:1.22.5 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY ./src ./src
COPY Makefile .
RUN make build

FROM alpine:latest
WORKDIR /
COPY --from=builder /app/build/ktool .
CMD ["sh"]