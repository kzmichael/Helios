# --- STAGE 1: Build the application ---
FROM node:22-alpine AS builder

WORKDIR /src

RUN corepack enable && corepack prepare pnpm@12.8.2 --activate

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .
RUN pnpm run build

RUN pnpm prune --prod



# --- STAGE 2: Run the production application ---
FROM gcr.io/distroless/nodejs22-debian12:nonroot AS runner

WORKDIR /src

COPY package*.json ./


# Copy the compiled JavaScript files from the builder stage
COPY --from=builder /src/dist ./dist
COPY --from=builder /src/node_modules ./node_modules


EXPOSE 3000

CMD ["dist/main.js"]
