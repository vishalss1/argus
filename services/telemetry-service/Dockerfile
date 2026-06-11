FROM golang:1.26.3-alpine AS build

WORKDIR /src

# Copy workspace files
COPY shared/ ./shared/
COPY services/telemetry-service/ ./services/telemetry-service/

# Build telemetry-service
WORKDIR /src/services/telemetry-service
RUN GOWORK=off CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetry-service ./cmd/main.go

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
	&& addgroup -S argus \
	&& adduser -S -G argus argus

WORKDIR /app

COPY --from=build /out/telemetry-service /app/telemetry-service

USER argus

EXPOSE 50052

ENTRYPOINT ["/app/telemetry-service"]
