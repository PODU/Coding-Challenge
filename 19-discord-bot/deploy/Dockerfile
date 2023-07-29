# ---- build stage ----
FROM node:20-slim AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY tsconfig.json ./
COPY src ./src
RUN npm run build

# ---- runtime stage ----
FROM node:20-slim AS runtime
ENV NODE_ENV=production
WORKDIR /app
COPY package*.json ./
RUN npm ci --omit=dev && npm cache clean --force
COPY --from=build /app/dist ./dist
COPY seed ./seed
# Catalog is written here at runtime; mount a volume to persist it.
RUN mkdir -p data && chown -R node:node /app
USER node
# DISCORD_TOKEN is provided at runtime via -e / --env-file / secret.
CMD ["node", "dist/index.js"]
