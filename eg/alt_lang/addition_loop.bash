#!/usr/bin/bash

# test - additional loop

target=4000000
a=0

for (( f=0; f<=$target ; f++ ))
do
    let a+=f
done

echo "${f} ${a}"

