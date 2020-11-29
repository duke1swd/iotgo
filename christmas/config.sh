#!/bin/bash
while read -a line
do
	echo mosquitto_pub -r -t ${line[0]} -m ${line[1]}
	mosquitto_pub -r -t ${line[0]} -m ${line[1]}
done << EOF
christmas/indoor/control auto
christmas/indoor/state on
christmas/season/start 11/1
christmas/season/end 1/6
christmas/enable true
christmas/indoor/window-start light
christmas/indoor/window-end 1:00
christmas/indoor/devices plug-0001
EOF
