# Nicholas Wang

FROM alpine:3.6 

RUN apk --no-cache add ca-certificates tzdata openssl\
 && ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime 

COPY ./etc/ /etc/

CMD /etc/telegraf/telegraf-controller
