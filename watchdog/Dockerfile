# Stage 1: Build the Go application
FROM golang:alpine AS builder

## Set the working directory
WORKDIR /app

## Copy the source code into the container
COPY . .

ENV GOPROXY=https://goproxy.cn

## Build the Go application
RUN go build -o watchdog .

# Stage 2: Create a minimal runtime image
FROM alpine:latest

## Set the working directory
WORKDIR /app/

## Copy the binary from the builder stage
COPY --from=builder /app/watchdog .

## Command to run the application
CMD ["/app/watchdog"]
