# Stage 1: Build the Go binary
FROM golang:1.26-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /app/bin/job ./cmd/job

# Stage 2: Runtime image with Playwright driver & Chromium browser
FROM mcr.microsoft.com/playwright:v1.62.1-jammy

WORKDIR /app

# Assemble Playwright-Go driver v1.50.1 from npm and preinstall Chromium with all OS dependencies
RUN mkdir -p /root/.cache/ms-playwright-go/1.50.1 && \
    curl -fsSL https://registry.npmjs.org/playwright-core/-/playwright-core-1.50.1.tgz | tar -xz -C /root/.cache/ms-playwright-go/1.50.1 && \
    cp "$(which node)" /root/.cache/ms-playwright-go/1.50.1/node && \
    /root/.cache/ms-playwright-go/1.50.1/node /root/.cache/ms-playwright-go/1.50.1/package/cli.js install --with-deps chromium

# Copy compiled binary from builder
COPY --from=builder /app/bin/job /app/job

# Environment defaults
ENV HEADLESS=true
ENV DRY_RUN=true
ENV SCREENSHOTS_DIR=/tmp/form-submission-screenshots

ENTRYPOINT ["/app/job"]
