#!/bin/bash
while read -a line
do
	echo mosquitto_pub -r -t ${line[0]} -m ${line[1]}
	mosquitto_pub -r -t ${line[0]} -m ${line[1]}
done << EOF
lighting/enable true
lighting/indoor/window-start light
lighting/indoor/window-end 23:00
lighting/indoor/devices tp-plug-03,tp-plug-04
lighting/indoor/season/start 11/1
lighting/indoor/season/end 1/6
lighting/outdoor/season/start 11/1
lighting/outdoor/season/end 1/6
lighting/outdoor/window-start light
lighting/outdoor/window-end 23:00
lighting/outdoor/devices plug-0002
lighting/carolinaroom/window-end 22:00
lighting/carolinaroom/devices tp-plug-01,tp-plug-02
EOF
