#!/usr/bin/bash

# test - additional loop

target=500000
a=0

for (( f=0; f<=$target ; f++ ))
do
    let a+=f
done
#    (( a=a+f ))

echo "${f} ${a}"

