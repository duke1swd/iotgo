#!/bin/bash
while read -a line
do
	echo mosquitto_pub -r -t ${line[0]} -m ${line[1]}
	mosquitto_pub -r -t ${line[0]} -m ${line[1]}
done << EOF
christmas/outdoor/command off
EOF
#devices/plug-0002/button/button true
#christmas/enable true
#christmas/test-region/devices plug-fake
#christmas/test-region/drop true