# KREJT backend — imazh i vetëm, dy binarë (api, worker); zgjidhet me BUILD ARG.
# Ndërtim për ARM64 (Fargate Graviton): docker buildx build --platform linux/arm64 --build-arg CMD=api .
#
# Faza e ndërtimit ecën në platformën e ndërtuesit (amd64 në CI) dhe kryqëzon drejt ARM64. Pa këtë,
# përpiluesi i Go-s vetë ekzekutohet nën emulim QEMU dhe ndërtimi zgjat shumëfish; me CGO të fikur
# kryqëzimi është i plotë dhe binari i dalë është i njëjti.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG CMD=api
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Versioni nuk hyn te binari: serveri e lexon nga APP_VERSION në kohë ekzekutimi, ndaj një imazh i
# vetëm mund të ripërdoret dhe deploy-i nuk detyrohet ta rindërtojë vetëm për etiketën.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${CMD}

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -u 10001 krejt
USER krejt
WORKDIR /app
COPY --from=build /out/app /app/app
ENV HTTP_PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/app"]
