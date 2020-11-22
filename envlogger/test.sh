#!/bin/sh
LOGDIR=.
export LOGDIR
MQTTBROKER=tcp://DanielPi3.local:1883
export MQTTBROKER
INTERVAL=1
export INTERVAL
./envlogger
