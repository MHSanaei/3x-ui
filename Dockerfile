FROM golang:alpine AS builder
ENV CGO_ENABLED 1
RUN apk add gcc && apk --no-cache --update add build-base
ENV DST_DIR=/opt/x-ui
ENV TMP_DIR=/tmp/x-ui

WORKDIR ${TMP_DIR}
COPY . .
RUN go build main.go

FROM alpine:latest
ENV DST_DIR=/opt/x-ui
ENV TMP_DIR=/tmp/x-ui
WORKDIR ${DST_DIR}
COPY --from=builder ${TMP_DIR}/main ${DST_DIR}/x-ui
COPY --from=builder ${TMP_DIR}/bin ${DST_DIR}/bin

VOLUME [ "/etc/x-ui" ]
CMD ["./x-ui","run"]
