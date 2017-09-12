FROM alpine
RUN apk add --update ca-certificates

ADD drone-cron /usr/bin/
ENTRYPOINT ["/usr/bin/drone-cron"]
