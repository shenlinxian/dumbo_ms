#!/bin/bash
#$1:nodes num
#$2:s num
#cat ips.txt | while read y
id=1
fault=$1
file="drtips.txt"

file="ip.txt"
ips=$(<"$file")
for ip in $ips
do
    if ((id > fault)); then
        #echo "Exiting loop because i ($i) is greater than n ($n)"
        break
    fi
    (ssh -o StrictHostKeyChecking=no -i chenghao.pem ubuntu@${ip} -tt "pkill fin_test")&
    let id++
done
