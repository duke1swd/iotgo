#!/bin/bash
while read -a line
do
	echo mosquitto_pub -r -t ${line[0]} -m ${line[1]}
	mosquitto_pub -r -t ${line[0]} -m ${line[1]}
done << EOF
lighting/outdoor/command off
EOF
#devices/plug-0002/button/button true
#lighting/enable true
#lighting/test-region/devices plug-fake
#lighting/test-region/drop true
