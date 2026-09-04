# KREJT — paneli i partnerit (Next.js, output standalone) për ECS Fargate ARM64.
# Ndërtimi nuk kërkon sekrete: adresa e API-së (KREJT_API_BASE_URL) lexohet në kohë ekzekutimi.
# Faza e ndërtimit ecën në platformën e ndërtuesit (amd64 në CI): Next.js/SWC nën emulim
# QEMU arm64 rrëzohet me SIGILL. Dalja standalone është JS i pastër, ndaj kopjohet në arm64.
FROM --platform=$BUILDPLATFORM node:24-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY . .
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:24-alpine
RUN apk add --no-cache wget && adduser -D -u 10001 krejt
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
COPY --from=build --chown=krejt:krejt /app/.next/standalone ./
COPY --from=build --chown=krejt:krejt /app/.next/static ./.next/static
USER krejt
EXPOSE 3000
CMD ["node", "server.js"]
