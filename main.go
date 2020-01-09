package main

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	health "github.com/Financial-Times/go-fthealth/v1_1"
	loggerv1 "github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/http-handlers-go/httphandlers"
	consumer "github.com/Financial-Times/message-queue-gonsumer"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	cli "github.com/jawher/mow.cli"
	"github.com/rcrowley/go-metrics"

	loggerv2 "github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/message-queue-go-producer/producer"
	"github.com/Financial-Times/post-publication-combiner/v2/processor"
	"github.com/Financial-Times/post-publication-combiner/v2/utils"
)

const serviceName = "post-publication-combiner"

func main() {

	app := cli.App(serviceName, "Service listening to content and metadata PostPublication events, and forwards a combined message to the queue")

	port := app.String(cli.StringOpt{
		Name:   "port",
		Value:  "8080",
		Desc:   "Port to listen on",
		EnvVar: "PORT",
	})
	contentTopic := app.String(cli.StringOpt{
		Name:   "contentTopic",
		Value:  "PostPublicationEvents",
		EnvVar: "KAFKA_CONTENT_TOPIC_NAME",
	})
	metadataTopic := app.String(cli.StringOpt{
		Name:   "metadataTopic",
		Value:  "PostConceptAnnotations",
		EnvVar: "KAFKA_METADATA_TOPIC_NAME",
	})
	combinedTopic := app.String(cli.StringOpt{
		Name:   "combinedTopic",
		Value:  "CombinedPostPublicationEvents",
		EnvVar: "KAFKA_COMBINED_TOPIC_NAME",
	})
	forcedCombinedTopic := app.String(cli.StringOpt{
		Name:   "forcedCombinedTopic",
		Value:  "ForcedCombinedPostPublicationEvents",
		EnvVar: "KAFKA_FORCED_COMBINED_TOPIC_NAME",
	})
	kafkaProxyAddress := app.String(cli.StringOpt{
		Name:   "kafkaProxyAddress",
		Value:  "http://localhost:8080",
		Desc:   "Address used by the queue consumer and producer to connect to the queue",
		EnvVar: "KAFKA_PROXY_ADDR",
	})
	kafkaContentConsumerGroup := app.String(cli.StringOpt{
		Name:   "kafkaContentTopicConsumerGroup",
		Value:  "content-post-publication-combiner",
		Desc:   "Group used to read the messages from the content queue",
		EnvVar: "KAFKA_PROXY_CONTENT_CONSUMER_GROUP",
	})
	kafkaMetadataConsumerGroup := app.String(cli.StringOpt{
		Name:   "kafkaMetadataTopicConsumerGroup",
		Value:  "metadata-post-publication-combiner",
		Desc:   "Group used to read the messages from the metadata queue",
		EnvVar: "KAFKA_PROXY_METADATA_CONSUMER_GROUP",
	})
	kafkaProxyRoutingHeader := app.String(cli.StringOpt{
		Name:   "kafkaProxyHeader",
		Value:  "kafka",
		Desc:   "Kafka proxy header - used for vulcan routing.",
		EnvVar: "KAFKA_PROXY_HOST_HEADER",
	})

	docStoreAPIBaseURL := app.String(cli.StringOpt{
		Name:   "docStoreApiBaseURL",
		Value:  "http://localhost:8080/__document-store-api",
		Desc:   "The address that the document store can be reached at. Important for content retrieval.",
		EnvVar: "DOCUMENT_STORE_BASE_URL",
	})
	docStoreAPIEndpoint := app.String(cli.StringOpt{
		Name:   "docStoreApiEndpoint",
		Value:  "/content/{uuid}",
		Desc:   "The endpoint used for content retrieval.",
		EnvVar: "DOCUMENT_STORE_API_ENDPOINT",
	})
	publicAnnotationsAPIBaseURL := app.String(cli.StringOpt{
		Name:   "publicAnnotationsApiBaseURL",
		Value:  "http://localhost:8080/__public-annotations-api",
		Desc:   "The address that the public-annotations-api can be reached at. Important for metadata retrieval.",
		EnvVar: "PUBLIC_ANNOTATIONS_API_BASE_URL",
	})
	publicAnnotationsAPIEndpoint := app.String(cli.StringOpt{
		Name:   "publicAnnotationsApiEndpoint",
		Value:  "/content/{uuid}/annotations",
		Desc:   "The endpoint used for metadata retrieval.",
		EnvVar: "PUBLIC_ANNOTATIONS_API_ENDPOINT",
	})
	whitelistedMetadataOriginSystemHeaders := app.Strings(cli.StringsOpt{
		Name:   "whitelistedMetadataOriginSystemHeaders",
		Value:  []string{"http://cmdb.ft.com/systems/pac", "http://cmdb.ft.com/systems/methode-web-pub", "http://cmdb.ft.com/systems/next-video-editor"},
		Desc:   "Origin-System-Ids that are supported to be processed from the PostPublicationEvents queue.",
		EnvVar: "WHITELISTED_METADATA_ORIGIN_SYSTEM_HEADERS",
	})
	whitelistedContentUris := app.Strings(cli.StringsOpt{
		Name:   "whitelistedContentURIs",
		Value:  []string{"methode-article-mapper", "wordpress-article-mapper", "next-video-mapper", "upp-content-validator"},
		Desc:   "Space separated list with content URI substrings - to identify accepted content types.",
		EnvVar: "WHITELISTED_CONTENT_URIS",
	})
	whitelistedContentTypes := app.Strings(cli.StringsOpt{
		Name:   "whitelistedContentTypes",
		Value:  []string{"Article", "Video", "MediaResource", "Audio", ""},
		Desc:   "Space separated list with content types - to identify accepted content types.",
		EnvVar: "WHITELISTED_CONTENT_TYPES",
	})

	loggerv1.InitDefaultLogger(serviceName)
	logConf := loggerv2.KeyNamesConfig{KeyTime: "@time"}
	logger := loggerv2.NewUPPLogger(serviceName, "INFO", logConf)

	app.Action = func() {
		client := http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConnsPerHost:   20,
				TLSHandshakeTimeout:   3 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		}

		// create channel for holding the post publication content and metadata messages
		messagesCh := make(chan *processor.KafkaQMessage, 100)

		// consume messages from content queue
		cConf := consumer.QueueConfig{
			Addrs: []string{*kafkaProxyAddress},
			Group: *kafkaContentConsumerGroup,
			Topic: *contentTopic,
			Queue: *kafkaProxyRoutingHeader,
		}
		cc := processor.NewKafkaQConsumer(cConf, messagesCh, &client, logger)
		go cc.Consumer.Start()
		defer cc.Consumer.Stop()

		// consume messages from metadata queue
		mConf := consumer.QueueConfig{
			Addrs: []string{*kafkaProxyAddress},
			Group: *kafkaMetadataConsumerGroup,
			Topic: *metadataTopic,
			Queue: *kafkaProxyRoutingHeader,
		}
		mc := processor.NewKafkaQConsumer(mConf, messagesCh, &client, logger)
		go mc.Consumer.Start()
		defer mc.Consumer.Stop()

		// process and forward messages
		dataCombiner := processor.NewDataCombiner(utils.ApiURL{BaseURL: *docStoreAPIBaseURL, Endpoint: *docStoreAPIEndpoint},
			utils.ApiURL{BaseURL: *publicAnnotationsAPIBaseURL, Endpoint: *publicAnnotationsAPIEndpoint}, &client)

		pQConf := processor.NewProducerConfig(*kafkaProxyAddress, *combinedTopic, *kafkaProxyRoutingHeader)
		msgProducer := producer.NewMessageProducerWithHTTPClient(pQConf, &client)
		processorConf := processor.NewMsgProcessorConfig(
			*whitelistedContentUris,
			*whitelistedMetadataOriginSystemHeaders,
			*contentTopic,
			*metadataTopic,
		)
		msgProcessor := processor.NewMsgProcessor(
			messagesCh,
			processorConf,
			dataCombiner,
			msgProducer,
			*whitelistedContentTypes)
		go msgProcessor.ProcessMessages()

		// process requested messages - used for reindexing and forced requests
		forcedPQConf := processor.NewProducerConfig(*kafkaProxyAddress, *forcedCombinedTopic, *kafkaProxyRoutingHeader)
		forcedMsgProducer := producer.NewMessageProducerWithHTTPClient(forcedPQConf, &client)
		requestProcessor := processor.NewRequestProcessor(
			dataCombiner,
			forcedMsgProducer,
			*whitelistedContentTypes)

		// Since the health check for all producers and consumers just checks /topics for a response, we pick a producer and a consumer at random
		routeRequests(port, &requestHandler{requestProcessor: requestProcessor}, NewCombinerHealthcheck(msgProducer, mc.Consumer, &client, *docStoreAPIBaseURL, *publicAnnotationsAPIBaseURL))
	}

	loggerv1.Infof("PostPublicationCombiner is starting with args %v", os.Args)

	err := app.Run(os.Args)
	if err != nil {
		loggerv1.WithError(err).Error("App could not start")
	}
}

func routeRequests(port *string, requestHandler *requestHandler, healthService *HealthcheckHandler) {
	r := http.NewServeMux()

	r.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)
	r.HandleFunc(status.PingPath, status.PingHandler)
	r.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(healthService.GTG))

	checks := []health.Check{
		checkKafkaProxyProducerConnectivity(healthService),
		checkKafkaProxyConsumerConnectivity(healthService),
		checkDocumentStoreAPIHealthcheck(healthService),
		checkPublicAnnotationsAPIHealthcheck(healthService),
	}

	hc := health.TimedHealthCheck{
		HealthCheck: health.HealthCheck{
			SystemCode:  "upp-post-publication-combiner",
			Name:        "post-publication-combiner",
			Description: "Checks for service dependencies: document-store, public-annotations-api, kafka proxy and the presence of related topics",
			Checks:      checks,
		},
		Timeout: 10 * time.Second,
	}

	r.Handle("/__health", handlers.MethodHandler{"GET": http.HandlerFunc(health.Handler(hc))})

	servicesRouter := mux.NewRouter()
	servicesRouter.HandleFunc("/{id}", requestHandler.postMessage).Methods("POST")

	var monitoringRouter http.Handler = servicesRouter
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(loggerv1.Logger(), monitoringRouter)
	monitoringRouter = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, monitoringRouter)

	r.Handle("/", monitoringRouter)

	server := &http.Server{Addr: ":" + *port, Handler: r}

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			loggerv1.Infof("HTTP server closing with message: %v", err)
		}
		wg.Done()
	}()

	waitForSignal()
	loggerv1.Infof("[Shutdown] PostPublicationCombiner is shutting down")

	if err := server.Close(); err != nil {
		loggerv1.WithError(err).Error("Unable to stop http server")
	}

	wg.Wait()
}

func waitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
