# Nicholas Wang

FROM harbor.aibyte.net/library/centos:7

#RUN apk --no-cache add ca-certificates tzdata openssl\
RUN ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime 

COPY ./etc/ /etc/

CMD /etc/telegraf/telegraf-controller
