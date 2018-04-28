FROM golang:alpine AS build-env
WORKDIR /app
ENV SRC_DIR=/go/src/github.com/shashidharatd/federation-dns
ADD . $SRC_DIR
RUN cd $SRC_DIR && go build -o /app/federation-dns cmd/federation-dns/main.go

FROM alpine
RUN apk update && \
   apk add --no-cache ca-certificates && \
   update-ca-certificates && \
   rm -rf /var/cache/apk/*
WORKDIR /app
COPY --from=build-env /app/federation-dns /app
ENTRYPOINT ./federation-dns
