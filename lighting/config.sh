#!/bin/bash
while read -a line
do
	echo mosquitto_pub -r -t ${line[0]} -m ${line[1]}
	mosquitto_pub -r -t ${line[0]} -m ${line[1]}
done << EOF
lighting/indoor/control auto
lighting/indoor/state on
lighting/season/start 11/1
lighting/season/end 1/6
lighting/enable true
lighting/indoor/window-start light
lighting/indoor/window-end 23:00
lighting/indoor/devices plug-0001,plug-0003
lighting/outdoor/window-start light
lighting/outdoor/window-end 23:00
lighting/outdoor/devices plug-0002
EOF
