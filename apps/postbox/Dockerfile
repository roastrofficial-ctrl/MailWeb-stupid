FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -o /postbox ./cmd/postbox

FROM alpine:3.21
RUN adduser -D -u 10001 postbox
COPY --from=build /postbox /usr/local/bin/postbox
USER postbox
EXPOSE 8080
ENTRYPOINT ["postbox"]

