#!/bin/bash
while read -a line
do
	echo mosquitto_pub -r -t ${line[0]} -m ${line[1]}
	mosquitto_pub -r -t ${line[0]} -m ${line[1]}
done << EOF
lighting/tree/season/start 11/1
lighting/tree/season/end 1/6
lighting/tree/window-start light
lighting/tree/window-end 23:00
lighting/tree/devices plug-0003
EOF
#lighting/enable true
#lighting/carolinaroom/window-end 22:00
#lighting/carolinaroom/devices tp-plug-03,tp-plug-02,tp-plug-04
#lighting/carolinaroom/window-start light
# lighting/indoor/window-start light
# lighting/indoor/window-end 23:00
# lighting/indoor/devices tp-plug-03,plug-0001
# lighting/indoor/season/start 11/1
# lighting/indoor/season/end 1/6
