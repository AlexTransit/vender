#!/bin/bash

fn="/run/user/1000/hb"
if [ ! -f $fn ]; then
 # application wacthdog not set
 exit 0
fi

hb=$( cat $fn )
echo $hb

if [ $hb -le 0 ]
then
 logger -p3 the app did not feed the watchdog
 exit 255
else
 echo $(($hb-1)) > $fn
 exit 0
fi
