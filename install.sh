#!/bin/bash

rm -r dist/
mkdir dist

yarn install
yarn build

go build -o dist/gpx_my_plugin_linux_amd64 -a -v pkg/main.go

rsync -auv --delete . /var/lib/grafana/plugins/simon-myplugin-datasource/