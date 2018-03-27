# upp-post-publication-combiner

[![Circle CI](https://circleci.com/gh/Financial-Times/post-publication-combiner/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/post-publication-combiner/tree/master)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/post-publication-combiner)](https://goreportcard.com/report/github.com/Financial-Times/post-publication-combiner) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/post-publication-combiner/badge.svg)](https://coveralls.io/github/Financial-Times/post-publication-combiner)

## Introduction
This service builds combined messages (content + annotations) based on events received from PostConceptAnnotations or PostPublicationEvents.  
The combined message is then sent to the CombinedPostPublicationEvents kafka queue.

This is a combination point for synchronizing the content and the metadata publish flows.
Note: one publish event can result in two messages in the CombinedPostPublicationEvents topics (one for the content publish, and one for the metadata publish).

This service depends on the following services:
- kafka/kafka-proxy
- document-store-api (/content endpoint)
- public-annotations-api (/content/{uuid}/annotations/{platformVersion} endpoint)

## Installation

In order to install, execute the following steps:

        go get -u github.com/kardianos/govendor
        go get -u github.com/Financial-Times/post-publication-combiner
        cd $GOPATH/src/github.com/Financial-Times/post-publication-combiner
        govendor sync
        go build .

## Running locally

1. Run the tests and install the binary:

        go test ./...
        go install

1. Run the binary (using the `help` flag to see the available optional arguments):

        $GOPATH/bin/post-publication-combiner

Please check --help for more details.

Test:
    You can verify the service's behavior by checking the consumed and the generated kafka messages.
    You can also use the [force enpoint](#force)

## Build and deployment

* Built by Docker Hub (from master or from github tags): [coco/post-publication-combiner](https://hub.docker.com/r/coco/post-publication-combiner/)
* CI provided by CircleCI: [post-publication-combiner](https://circleci.com/gh/Financial-Times/post-publication-combiner)

## Service/Utility endpoints
<a name="force">Force endpoint</a>

`/{uuid}`
###POST
Creates and forwards a CombinedPostPublicationEvent to the queue for the provided UUID.
Request body should be empty. 
The combiner reads the content with that UUID from document-store, and based on its content type, it complements the message with the corresponding annotations.

If the force request has the `X-Request-Id` header set, that value will be propagated to the queue - as a message header.

Returns 200 if the message was published successfully

Returns 422 (Unprocessable Entity) for an invalid uuid

Returns 404 for missing content and metadata for the provided uuid

Returns 500 for unexpected processing errors

## Healthchecks
Our standard admin endpoints are:
`/__gtg` - returns 503 if any if the checks executed at the /__health endpoint returns false

`/__health`

Checks if:
* kafka-proxy is reachable
* document-store-api is reachable
* public-annotations-api is reachable

`/__build-info` 

### Logging

* The application uses [logrus](https://github.com/sirupsen/logrus); the log file is initialised in [main.go](main.go).
* NOTE: `/__build-info` and `/__gtg` endpoints are not logged as they are called every second from varnish/vulcand and this information is not needed in logs/splunk.