FROM node:20-alpine AS builder

WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci --ignore-scripts=false
COPY tsconfig.json ./
COPY src/ ./src/
RUN npm run build

FROM node:20-alpine
RUN adduser -D -g '' appuser
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
COPY package.json ./
USER appuser
EXPOSE 3001
CMD ["node", "dist/index.js"]
