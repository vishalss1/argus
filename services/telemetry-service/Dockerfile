FROM golang:1.26.3-bookworm AS build

WORKDIR /src

# Copy workspace files
COPY shared/ ./shared/
COPY services/telemetry-service/ ./services/telemetry-service/

# Build telemetry-service
WORKDIR /src/services/telemetry-service
RUN GOWORK=off CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetry-service ./cmd/main.go

# Model download stage
FROM alpine:3.22@sha256:310c62b5e7ca5b08167e4384c68db0fd2905dd9c7493756d356e893909057601 AS model-downloader
RUN apk add --no-cache curl
WORKDIR /models/all-MiniLM-L6-v2
RUN curl -sL -o model.onnx "https://huggingface.co/Xenova/all-MiniLM-L6-v2/resolve/main/onnx/model.onnx"
RUN echo "759c3cd2b7fe7e93933ad23c4c9181b7396442a2ed746ec7c1d46192c469c46e  model.onnx" | sha256sum -c -
RUN curl -sL -o tokenizer.json "https://huggingface.co/Xenova/all-MiniLM-L6-v2/resolve/main/tokenizer.json"
RUN curl -sL -o config.json "https://huggingface.co/Xenova/all-MiniLM-L6-v2/resolve/main/config.json"

# ONNX Runtime download stage
FROM alpine:3.22@sha256:310c62b5e7ca5b08167e4384c68db0fd2905dd9c7493756d356e893909057601 AS onnx-downloader
RUN apk add --no-cache curl tar
WORKDIR /onnx
RUN curl -sL -o onnxruntime.tgz "https://github.com/microsoft/onnxruntime/releases/download/v1.23.0/onnxruntime-linux-x64-1.23.0.tgz"
RUN tar -xzf onnxruntime.tgz

FROM debian:bookworm-slim@sha256:96e378d7e6531ac9a15ad505478fcc2e69f371b10f5cdf87857c4b8188404716

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget \
	&& groupadd -r argus && useradd -r -g argus argus \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=build /out/telemetry-service /app/telemetry-service
COPY --from=model-downloader /models /app/models
COPY --from=onnx-downloader /onnx/onnxruntime-linux-x64-1.23.0/lib/libonnxruntime.so.1.23.0 /usr/lib/libonnxruntime.so

USER argus

EXPOSE 50052

ENTRYPOINT ["/app/telemetry-service"]
