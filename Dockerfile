FROM golang:alpine AS build
ARG appVersion=0.0.0
RUN apk add --update --no-cache ca-certificates git
COPY . $GOPATH/src/github.com/Ahton89/r2s_v2/
WORKDIR $GOPATH/src/github.com/Ahton89/r2s_v2/
RUN go get -d -v
RUN cd cmd/r2s_v2/ && GOARCH=amd64 CGO_ENABLED=0 GOOS=linux go build -ldflags="-X 'main.appVersion=$appVersion'" -o /go/bin/r2s
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/bin/r2s /bin/
ENTRYPOINT ["r2s"]