FROM node:22.23.3-trixie-slim AS frontend
WORKDIR /build/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM golang:1.27.2-trixie AS backend
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -trimpath -o /out/jst ./cmd/server

FROM debian:trixie-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=backend /out/jst /app/jst
COPY --from=frontend /build/web/dist/ /app/web

USER 501:20
EXPOSE 8080
ENTRYPOINT ["/app/jst"]