FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/core-api ./services/core-api/cmd/core-api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/core-api /usr/local/bin/core-api
ENTRYPOINT ["core-api"]
