#! /bin/sh
set -e

trap -- '' SIGKILL SIGINT SIGTERM 
case "$1" in
    native)
        ID=$(ps -ef | grep telegraf | grep etc | grep -v grep | grep -v sh | awk '{print $2}') 
        echo $ID
        for id in $ID
        do
        kill -9 $id
        echo "killed $id"
        done
        echo "test"
            
        /etc/telegraf/telegraf -config /etc/telegraf/telegraf.conf -config-directory /etc/telegraf/telegraf.d > /etc/telegraf/telegraf_controller.log 2>&1 &
        ;;
    *)
        echo "Unsupported reload strategy: $1"
        exit 1
        ;;
esac
