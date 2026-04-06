# Agent Runtime — Node.js container with Pi SDK
# Two entry points: agent-server.js (daemon) and agent-task.js (task)

FROM node:22-slim AS builder

WORKDIR /app

# Install dependencies
COPY package.json package-lock.json* ./
RUN npm ci --ignore-scripts

# Copy source and build
COPY tsconfig.json ./
COPY src/ ./src/
RUN npm run build

# Production image
FROM node:22-slim

WORKDIR /app

# Install production dependencies only
COPY package.json package-lock.json* ./
RUN npm ci --omit=dev --ignore-scripts && npm cache clean --force

# Copy built output
COPY --from=builder /app/dist ./dist

# Create data directories
RUN mkdir -p /data/sessions /data/repos /data/scratch \
    && chown -R 1000:1000 /data

USER 1000:1000

# Default: daemon mode. Override CMD for task mode.
CMD ["node", "/app/dist/agent-server.js"]
