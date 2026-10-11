# Od Supervisora 2026.04.0 BUILD_FROM nie jest podawany domyślnie — obrazy bazowe wpisane jawnie.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/obowiazki ./cmd/obowiazki

# Runtime bez dodatkowych pakietów: appka łączy się tylko z http://supervisor (bez TLS),
# a dane stref czasowych są wbudowane w binarkę (time/tzdata).
FROM alpine:3.22
COPY --from=build /out/obowiazki /usr/local/bin/obowiazki
EXPOSE 8099
CMD ["/usr/local/bin/obowiazki"]
