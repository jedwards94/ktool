# Stage 1: Build the Go application
ARG GO_IMAGE=golang:1.22.5

FROM ${GO_IMAGE} AS builder
ARG TARGET

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY ./src ./src
COPY Makefile .
RUN make build TARGET=$TARGET;

FROM alpine:latest
WORKDIR /
COPY --from=builder /app/build/* .
CMD ["sh"]