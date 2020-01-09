# upp-post-publication-combiner

[![Circle CI](https://circleci.com/gh/Financial-Times/post-publication-combiner/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/post-publication-combiner/tree/master)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/post-publication-combiner)](https://goreportcard.com/report/github.com/Financial-Times/post-publication-combiner) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/post-publication-combiner/badge.svg)](https://coveralls.io/github/Financial-Times/post-publication-combiner)

## Introduction
This service builds combined messages (content + annotations) based on events received from PostConceptAnnotations or PostPublicationEvents.  
The combined message is then sent to the CombinedPostPublicationEvents kafka queue.

This is a combination point for synchronizing the content and the metadata publish flows.
Note: one publish event can result in two messages in the CombinedPostPublicationEvents topics (one for the content publish, and one for the metadata publish).

The service has a force endpoint, that allows putting a combined message in the queue, with the actual data from our content and metadata stores.

This service depends on the following services:
- kafka/kafka-proxy
- document-store-api (/content endpoint)
- public-annotations-api (/content/{uuid}/annotations endpoint)

## Installation

Download the source code, dependencies and test dependencies:

        go get github.com/Financial-Times/post-publication-combiner
        cd $GOPATH/src/github.com/Financial-Times/post-publication-combiner
        go install

## Running locally

1. Run the tests and install the binary:

        go test ./...
        go install

1. Run the binary (using the `help` flag to see the available optional arguments):

        $GOPATH/bin/post-publication-combiner [--help]

Test:
    You can verify the service's behavior by checking the consumed and the generated kafka messages.
    You can also use the [force endpoint](#force)

## Build and deployment

* Built by Docker Hub (from master or from Github tags): [coco/post-publication-combiner](https://hub.docker.com/r/coco/post-publication-combiner/)
* CI provided by CircleCI: [post-publication-combiner](https://circleci.com/gh/Financial-Times/post-publication-combiner)

## Service/Utility endpoints

Refer to [api.yml](_ft/api.yml) for api related documentation.

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

* The application uses the FT [go-logger](https://github.com/Financial-Times/go-logger) library, based on [logrus](https://github.com/sirupsen/logrus).
* NOTE: `/__build-info` and `/__gtg` endpoints are not logged as they are called frequently.
