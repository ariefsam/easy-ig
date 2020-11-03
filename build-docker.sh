#!/bin/sh

env GOOS=linux GOARCH=amd64 go build -v

docker login repo.treescale.com
docker build -t repo.treescale.com/ariefsam/easy-ig:1.0.0 .

docker push repo.treescale.com/ariefsam/easy-ig:1.0.0