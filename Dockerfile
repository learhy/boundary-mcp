FROM alpine:latest

RUN apk add --no-cache ca-certificates

COPY boundary-mcp /usr/local/bin/boundary-mcp

ENTRYPOINT ["boundary-mcp"]