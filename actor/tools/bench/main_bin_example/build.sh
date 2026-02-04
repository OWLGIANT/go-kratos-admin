#!/bin/bash

EXECUTE_NAME="main_bin_example"

LDFlags="-w -s"

print_red() {
  echo -e "\e[31m$1\e[0m"
}

print_green() {
  echo -e "\e[32m$1\e[0m"
}

if [ ! -d "output" ]; then
  mkdir output
fi

if [ "$tags" != "" ];then
  GOOS=linux GOARCH=amd64 go build -ldflags "$LDFlags" -o ./output/$EXECUTE_NAME -tags $tags
else
  GOOS=linux GOARCH=amd64 go build -ldflags "$LDFlags" -o ./output/$EXECUTE_NAME
fi

if [ $? != 0 ]; then 
  print_red "FAILED TO BUILD"
  exit 1
fi

print_green "SUCCESS BUILD"


