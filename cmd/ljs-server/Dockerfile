
FROM golang:1.13-alpine AS build_base
LABEL maintainer="Aviv Laufer <aviv.laufer@gmail.com>"
RUN apk update && apk upgrade && \
    apk add --no-cache git build-base ca-certificates

WORKDIR /workdir
ENV GO111MODULE=on
COPY go.mod .
COPY go.sum .
RUN go mod download


FROM build_base AS builder
COPY /. .
WORKDIR /workdir
RUN cd /workdir &&  GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -ldflags "-w -s" -o /long_john_silver -tags netgo -installsuffix netgo .

FROM alpine
RUN apk add --no-cache ca-certificates

COPY --from=builder long_john_silver /bin/long_john_silver

ENTRYPOINT ["/bin/long_john_silver"]
