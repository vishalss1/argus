FROM golang:1.26.3-alpine AS build

WORKDIR /src

# Copy workspace files
COPY shared/ ./shared/
COPY services/core-service/ ./services/core-service/

# Build core-service
WORKDIR /src/services/core-service
RUN GOWORK=off CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/core-service ./cmd/api/main.go

FROM alpine:3.22

RUN apk add --no-cache ca-certificates curl \
	&& addgroup -S argus \
	&& adduser -S -G argus argus

WORKDIR /app

COPY --from=build /out/core-service /app/core-service
COPY migrations /app/migrations

USER argus

EXPOSE 8080 50051

ENTRYPOINT ["/app/core-service"]
