#!/bin/bash
#
# This script publishes a configuration file.
#
DEVICE=$1
if [ x${DEVICE} == x ] ; then
	echo Usage: config DEVICE 1>&2
	echo or: config DEVICE JSON 1>&2
	exit 1
fi

mqtt-clean -l -p devices/$DEVICE/\$implementation/config

JSON=$2
if [ x${JSON} == x ] ; then
	exit 0
fi

echo Publishing $JSON to $DEVICE
echo Press Enter to continue
read x


echo mosquitto_pub -t devices/$DEVICE/\$implementation/config/set -m `cat $JSON`
mosquitto_pub -t devices/$DEVICE/\$implementation/config/set -m "`cat $JSON`"
