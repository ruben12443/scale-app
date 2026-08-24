FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/scale-gateway ./services/scale-gateway/cmd/scale-gateway

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/scale-gateway /usr/local/bin/scale-gateway
ENTRYPOINT ["scale-gateway"]
