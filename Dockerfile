# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.26.4-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app ./cmd/api.go \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /canonicalize-emails ./cmd/canonicalize-emails

# ---- Runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /app /app
COPY --from=build /canonicalize-emails /canonicalize-emails

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app"]
