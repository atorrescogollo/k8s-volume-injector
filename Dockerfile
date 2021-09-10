FROM golang:1.16-alpine as build

WORKDIR /app
COPY . /app/

RUN go mod download
RUN CGO_ENABLED=0 go build cmd/main.go

FROM alpine:3.14.0 as run

COPY --from=build /app/main /k8s-volume-injector
COPY docker-entrypoint.sh /docker-entrypoint.sh

RUN chmod +x /docker-entrypoint.sh /k8s-volume-injector

ENTRYPOINT [ "/docker-entrypoint.sh" ]
