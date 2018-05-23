FROM labreg.arvancloud.com:5000/alpine:latest
RUN apk update && apk add libc6-compat
ADD redins /usr/bin
ADD config.json /CORE/redins/etc/
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
