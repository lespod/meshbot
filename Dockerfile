# syntax=docker/dockerfile:1

#######################
### Building container

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build
WORKDIR /app

ARG TARGETOS
ARG TARGETARCH

# Install dependencies
COPY go.mod go.sum .
RUN go mod download

# Copy source
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o output/meshbot .

######################
### Running container

FROM alpine:3.22 AS run
WORKDIR /app

# Copy the application executable from the build image
COPY --from=build /app/output /app

COPY ./config.json /app/default-config/config.json
COPY ./docker-entrypoint.sh /usr/local/bin/meshbot-entrypoint
RUN chmod +x /usr/local/bin/meshbot-entrypoint

VOLUME ["/app/config"]
ENTRYPOINT ["/usr/local/bin/meshbot-entrypoint"]
