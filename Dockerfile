FROM labreg.arvancloud.com:5000/alpine:latest
RUN apk update && apk add libc6-compat
ADD redins /usr/bin

ADD redins.ini /CORE/redins/etc/
