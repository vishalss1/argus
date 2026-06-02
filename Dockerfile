FROM golang:1.26.3-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/argus-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ai-worker ./cmd/ai-worker

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
	&& addgroup -S argus \
	&& adduser -S -G argus argus

WORKDIR /app

COPY --from=build /out/argus-api /app/argus-api
COPY --from=build /out/ai-worker /app/ai-worker
COPY migrations /app/migrations

USER argus

EXPOSE 8080

ENTRYPOINT ["/app/argus-api"]
