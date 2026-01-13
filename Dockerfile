# Use the official Golang image to build the app
# Keep this in sync with go.mod `go` version.
FROM golang:1.24 as builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# NOTE:
# gqlgen generated files are committed in this repo.
# Running codegen during Docker build can OOM/timeout in Cloud Build.
# If the GraphQL schema changes, run `go run github.com/99designs/gqlgen generate`
# locally and commit the updated generated files.

# Build binaries with static linking and target Linux amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o seed-dev-upgrade ./cmd/seed-dev-upgrade

# Start a new stage from scratch
FROM alpine:latest

# Install tzdata package
RUN apk add --no-cache tzdata

# Set the Current Working Directory inside the container
WORKDIR /root/

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/main .
COPY --from=builder /app/seed-dev-upgrade .

# Ensure the binary has execute permissions
RUN chmod +x ./main

# Cloud Run will route to container port 8080 by default.
EXPOSE 8080

# Command to run the executable
CMD ["./main"]
