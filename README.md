# r2s
[![Build Status](https://travis-ci.com/onetwotrip/r2s_v2.svg?branch=master)](https://travis-ci.org/onetwotrip/r2s)

Tool for copy hashes from redis to redis

## Run

#### ENV variables
|variable|required|default value|description|
|-------------|:-------------:|-----:|-------------|
|```REDIS_PRODUCTION_HOST```|false|127.0.0.1|prod redis host|
|```REDIS_PRODUCTION_PORT```|false|6379|prod redis port|
|```REDIS_PRODUCTION_DB```|false|0|num of prod database|
|```RECIPIENTS```|true|[]string|string of nodes hostnames with ',' separator e.g ```node1,node2,node_n```|
|```HASHES```|true|[]string|string of hashes with ',' separator|
|```RECIPIENT_REDIS_DB_NUM```|false|0|num of recipient database|
|```RECIPIENT_REDIS_PORT```|false|6379|recipient port of redis|
|```SSH_USERNAME```|true|-|username|
|```SSH_AUTH_SOCK```|true|-|path to ssh auth sock|
|```RECIPIENT_DOMAIN```|true|-|domain e.g ```example.com```|
|```DEBUG```|false|false|debug mode (more logs for each step)|
|```EXIT_IF_ERROR```|false|false|exit with exit code 1 if more one errors|
|```BUILD_URL```|false|```https://example.com```|if use jenkins - use ```${BUILD_URL}console``` variable|
|```SLACK_HOOK_URL```|true|-|slack hook for send alert|

NOTE: If you want overwrite recipient port and number of db - use ```hostname:port:db``` in ```RECIPIENTS``` env e.g ```node1,node2,node3:6379:1,node4:6380:2```

#### Example (Jenkins job)

```
#!/bin/bash

set -e 

VERSION="your_version"

RECIPIENT_=""
RECIPIENTS_LIST="node1,node2,node3,node4:6380"

if [ "$BUILD_CAUSE" == "TIMERTRIGGER" ]; then
  RECIPIENT_=$RECIPIENTS_LIST
  DEBUG=true
else
  RECIPIENT_=$recipient #jenkins choise parameter
fi

HASHES_TO_COPY="hash1,hash2"

SLACK_HOOK="https://hooks.slack.com/services/<secure_string>"

env
echo $REGISTRY_PASSWORD | docker login -u $REGISTRY_USERNAME --password-stdin your.registry.com
docker run --net=host --name $JOB_NAME-$BUILD_NUMBER --rm \
    -v /tmp:/tmp \
    -e SSH_AUTH_SOCK=$SSH_AUTH_SOCK \
    -e DEBUG=$DEBUG \
    -e EXIT_IF_ERROR=$EXIT_IF_ERROR \
    -e BUILD_URL="${BUILD_URL}console" \
    -e SLACK_HOOK_URL=$SLACK_HOOK \
    -e RECIPIENTS=$RECIPIENT_ \
    -e SSH_USERNAME="your_username" \
    -e REDIS_PRODUCTION_PORT=6379 \
    -e RECIPIENT_DOMAIN="example.com" \
    -e HASHES=$HASHES_TO_COPY \
    your.registry.com/r2s:$VERSION
```

