# --- STAGE 1: Build the application ---
FROM node:22-alpine AS builder

WORKDIR /app

ENV NEXT_PUBLIC_API_BASE=http://localhost:8080/v1
ENV NEXT_PUBLIC_WS_BASE=ws://localhost:8080/v1

RUN corepack enable && corepack prepare pnpm@12.8.2 --activate

COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .
RUN pnpm run build



# --- STAGE 2: Run the production application ---
FROM gcr.io/distroless/nodejs22-debian12:nonroot AS runner

WORKDIR /app

ENV HOSTNAME=0.0.0.0
ENV PORT=3000

# Copy the compiled JavaScript files from the builder stage
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

EXPOSE 3000

CMD ["server.js"]
