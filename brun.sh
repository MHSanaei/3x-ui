#!/bin/bash


CI_REGISTRY="reg.vados.ru"
#CI_BASE="$CI_REGISTRY/golang:1.26-alpine"
#CI_FROM="$CI_REGISTRY/alpine"
#CI_PROJECT="3x-ui"
#WORKDIR="/app"
#TARGETARCH="linux/amd64"
CI_IMAGE="$CI_REGISTRY/3xui_app"

#CI_BUILD_ENGINE="buildx"
#CI_BUILDX_BUILDER="buildx_buildkit_custom_3x-ui-ci"

#CI_TAGET="--push"
#--load"

#export $CI_REGISTRY $CI_BASE $CI_FROM $CI_PROJECT $WORKDIR $TARGETARCH $CI_IMAGE $CI_BUILD_ENGINE $CI_BUILDX_BUILDER

docker build --progress=plain --no-cache --push -t $CI_IMAGE -f Dockerfile .

#docker push $CI_IMAGE


#docker buildx create --name $CI_BUILDX_BUILDER --driver docker-container --use ;

#docker buildx build --builder $CI_BUILDX_BUILDER --progress=plain --platform $CI_BUILDX_PLATFORMS \
#                --build-arg CI_BASE_IMAGE=$CI_BASE_IMAGE $CI_TARGET -t $CI_IMAGE -f Dockerfile
