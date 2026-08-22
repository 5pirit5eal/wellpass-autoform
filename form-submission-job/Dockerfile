# Stage 1: Build the Go binary
FROM golang:1.26-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/bin/job ./cmd/job

# Stage 2: Runtime image with Playwright driver & Chromium browser
FROM golang:1.26-bookworm AS runner

WORKDIR /app

# Install Playwright driver, browser and all OS dependencies
COPY go.mod go.sum ./
RUN go mod download
RUN go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium

# Copy compiled binary from builder
COPY --from=builder /app/bin/job /app/job

# Environment defaults
ENV HEADLESS=true
ENV DRY_RUN=true
ENV SCREENSHOTS_DIR=/tmp/form-submission-screenshots

ENTRYPOINT ["/app/job"]
