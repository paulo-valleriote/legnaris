# syntax=docker/dockerfile:1

# ---- Build stage ----
# Node major matches web/.nvmrc (and the CI toolchain), so the image builds the
# same bundle the pipeline validates.
FROM node:24-alpine AS build
WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

# The API origin is not a build input: the runtime stage writes /config.json
# from API_BASE_URL at container start, so one image serves every environment.
RUN npm run build

# ---- Runtime stage ----
FROM nginx:1.29-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY docker-entrypoint.d/40-runtime-config.sh /docker-entrypoint.d/40-runtime-config.sh
RUN chmod +x /docker-entrypoint.d/40-runtime-config.sh

# Copying the whole dist directory rather than its assets keeps the service
# worker, the manifest, and the icons next to the shell they belong to.
COPY --from=build /src/dist /usr/share/nginx/html

EXPOSE 80
