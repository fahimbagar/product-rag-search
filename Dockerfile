FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/server ./cmd/server \
    && go build -o /out/migrate ./cmd/migrate \
    && go build -o /out/seed ./cmd/seed

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/server /out/migrate /out/seed /usr/local/bin/
COPY migrations /migrations
COPY seed/products.json /seed/products.json
