#!/bin/bash
#--project-name=amster-registry 
#docker network create -d bridge --attachable registry-ui-net

#docker compose up -d --rem
#docker compose registry-ui stop
#docker cp  registry-ui:/etc/nginx/nginx.conf tmp.conf
#ls -l tmp.conf

docker-compose build --pull --no-cache
#docker-compose logs registry-ui
docker compose up -d
