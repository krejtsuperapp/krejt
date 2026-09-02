# KREJT backend — imazh i vetëm, dy binarë (api, worker); zgjidhet me BUILD ARG.
# Ndërtim për ARM64 (Fargate Graviton): docker buildx build --platform linux/arm64 --build-arg CMD=api .
FROM golang:1.23-alpine AS build
ARG CMD=api
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${APP_VERSION:-dev}" -o /out/app ./cmd/${CMD}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -u 10001 krejt
USER krejt
WORKDIR /app
COPY --from=build /out/app /app/app
ENV HTTP_PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/app"]
