# Stage 1: Build the Go binary
FROM golang:1.26 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/bin/job ./cmd/job

# Stage 2: Runtime image with Playwright & Chromium dependencies
FROM mcr.microsoft.com/playwright:v1.49.1-jammy

WORKDIR /app

# Copy compiled binary from builder
COPY --from=builder /app/bin/job /app/job

# Environment defaults
ENV HEADLESS=true
ENV DRY_RUN=true
ENV SCREENSHOTS_DIR=/tmp/form-submission-screenshots

ENTRYPOINT ["/app/job"]
