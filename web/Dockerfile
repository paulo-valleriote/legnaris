# syntax=docker/dockerfile:1

# ---- Build stage ----
# Node major matches web/.nvmrc (and the CI toolchain), so the image builds the
# same bundle the pipeline validates.
FROM node:24-alpine AS build
WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

# Vite inlines every VITE_* value into the bundle at build time, so the API
# origin is a build argument, not a runtime environment: changing it requires
# rebuilding this image (`docker compose up -d --build web`).
ARG VITE_API_BASE_URL
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
RUN npm run build

# ---- Runtime stage ----
FROM nginx:1.29-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copying the whole dist directory rather than its assets keeps the service
# worker, the manifest, and the icons next to the shell they belong to.
COPY --from=build /src/dist /usr/share/nginx/html

EXPOSE 80
