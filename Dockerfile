FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod main.go watcher.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /x-reset-monitor .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S -G app app && mkdir -p /data && chown app:app /data
USER app
COPY --from=build /x-reset-monitor /usr/local/bin/x-reset-monitor
ENTRYPOINT ["/usr/local/bin/x-reset-monitor"]
