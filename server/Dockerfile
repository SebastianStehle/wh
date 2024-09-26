# syntax=docker/dockerfile:1

# Build the application from source
FROM golang:1.23-alpine AS build-stage

WORKDIR /app

# Download all the dependencies
COPY go.mod .
COPY go.sum .

# Install packages and templ compiler
RUN go mod download
RUN go install github.com/a-h/templ/cmd/templ@latest

COPY . .

# Compile templ
RUN templ generate

RUN CGO_ENABLED=0 GOOS=linux go build ./cmd/app/

# Deploy the application binary into a lean image

FROM alpine:3.9 AS build-release-stage

WORKDIR /app

COPY --from=build-stage /app/app /app
COPY --from=build-stage /app/configs/ /app/configs/
COPY --from=build-stage /app/public/ /app/public/

RUN ls -R

EXPOSE 5000

ENTRYPOINT ["./app"]