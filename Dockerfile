FROM golang:alpine AS build
ARG appVersion=0.0.0
RUN apk add --update --no-cache ca-certificates git
COPY . $GOPATH/src/github.com/onetwotrip/r2s_v2/
WORKDIR $GOPATH/src/github.com/onetwotrip/r2s_v2/cmd/r2s_v2/
RUN go get -d -v
RUN GOARCH=amd64 CGO_ENABLED=0 GOOS=linux go build -ldflags="-X 'main.appVersion=$appVersion'" -o /bin/r2s
ENTRYPOINT ["r2s"]