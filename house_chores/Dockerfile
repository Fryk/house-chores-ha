# Since Supervisor 2026.04.0 BUILD_FROM is no longer provided, so base images are set explicitly.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/house-chores ./cmd/house-chores

# No extra packages at runtime: the app only talks to http://supervisor (no TLS)
# and time zone data is embedded in the binary (time/tzdata).
FROM alpine:3.22
COPY --from=build /out/house-chores /usr/local/bin/house-chores
EXPOSE 8099
CMD ["/usr/local/bin/house-chores"]
