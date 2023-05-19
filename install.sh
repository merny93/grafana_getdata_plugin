#!/bin/bash

rm -r dist/
mkdir dist

yarn install
yarn build

go clean --cache
go build -o dist/gpx_my_plugin_linux_amd64 -v pkg/main.go

rsync -auv --delete . /var/lib/grafana/plugins/simon-dirfile-datasource/
