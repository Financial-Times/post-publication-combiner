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
	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/http-handlers-go/v2/httphandlers"
	"github.com/Financial-Times/kafka-client-go/v2"
	"github.com/Financial-Times/message-queue-gonsumer/consumer"
	"github.com/Financial-Times/post-publication-combiner/v2/processor"
	"github.com/Financial-Times/post-publication-combiner/v2/utils"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	cli "github.com/jawher/mow.cli"
	"github.com/rcrowley/go-metrics"
)

const (
	serviceName = "post-publication-combiner"
	systemCode  = "post-publication-combiner"
)

func main() {
	app := cli.App(serviceName, "Service listening to content and metadata PostPublication events, and forwards a combined message to the queue")

	logLevel := app.String(cli.StringOpt{
		Name:   "logLevel",
		Value:  "INFO",
		Desc:   "Logging level (DEBUG, INFO, WARN, ERROR)",
		EnvVar: "LOG_LEVEL",
	})
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
	kafkaAddress := app.String(cli.StringOpt{
		Name:   "kafkaAddress",
		Value:  "kafka:9092",
		Desc:   "Address used by the queue producer to connect to Kafka",
		EnvVar: "KAFKA_ADDR",
	})
	kafkaProxyAddress := app.String(cli.StringOpt{
		Name:   "kafkaProxyAddress",
		Value:  "http://localhost:8080",
		Desc:   "Address used by the queue consumer to connect to the queue",
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
	internalContentAPIBaseURL := app.String(cli.StringOpt{
		Name:   "internalContentApiBaseURL",
		Value:  "http://localhost:8080/__internal-content-api",
		Desc:   "The address that the internal-content-api can be reached at. Important for internal content and metadata retrieval.",
		EnvVar: "INTERNAL_CONTENT_API_BASE_URL",
	})
	internalContentAPIEndpoint := app.String(cli.StringOpt{
		Name:   "internalContentApiEndpoint",
		Value:  "/internalcontent/{uuid}?unrollContent=true",
		Desc:   "The endpoint used for internal content and metadata retrieval.",
		EnvVar: "INTERNAL_CONTENT_API_ENDPOINT",
	})
	contentCollectionRWBaseURL := app.String(cli.StringOpt{
		Name:   "contentCollectionRWBaseURL",
		Value:  "http://localhost:8080/__content-collection-rw-neo4j",
		Desc:   "The address that the content collection RW-er can be reached at. Important for content collection data retrieval.",
		EnvVar: "CONTENT_COLLECTION_RW_BASE_URL",
	})
	contentCollectionRWEndpoint := app.String(cli.StringOpt{
		Name:   "contentCollectionRWEndpoint",
		Value:  "/content-collection/content-package/{uuid}",
		Desc:   "The endpoint used for content collection data retrieval.",
		EnvVar: "CONTENT_COLLECTION_RW_ENDPOINT",
	})
	whitelistedMetadataOriginSystemHeaders := app.Strings(cli.StringsOpt{
		Name:   "whitelistedMetadataOriginSystemHeaders",
		Value:  []string{"http://cmdb.ft.com/systems/pac", "http://cmdb.ft.com/systems/next-video-editor"},
		Desc:   "Origin-System-Ids that are supported to be processed from the PostPublicationEvents queue.",
		EnvVar: "WHITELISTED_METADATA_ORIGIN_SYSTEM_HEADERS",
	})
	whitelistedContentUris := app.Strings(cli.StringsOpt{
		Name:   "whitelistedContentURIs",
		Value:  []string{"next-video-mapper", "upp-content-validator"},
		Desc:   "Space separated list with content URI substrings - to identify accepted content types.",
		EnvVar: "WHITELISTED_CONTENT_URIS",
	})
	whitelistedContentTypes := app.Strings(cli.StringsOpt{
		Name:   "whitelistedContentTypes",
		Value:  []string{"Article", "Video", "MediaResource", "Audio", ""},
		Desc:   "Space separated list with content types - to identify accepted content types.",
		EnvVar: "WHITELISTED_CONTENT_TYPES",
	})

	log := logger.NewUPPLogger(serviceName, *logLevel)

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
		cc := processor.NewKafkaQConsumer(cConf, messagesCh, &client)
		go cc.Consumer.Start()
		defer cc.Consumer.Stop()

		// consume messages from metadata queue
		mConf := consumer.QueueConfig{
			Addrs: []string{*kafkaProxyAddress},
			Group: *kafkaMetadataConsumerGroup,
			Topic: *metadataTopic,
			Queue: *kafkaProxyRoutingHeader,
		}
		mc := processor.NewKafkaQConsumer(mConf, messagesCh, &client)
		go mc.Consumer.Start()
		defer mc.Consumer.Stop()

		// process and forward messages
		dataCombiner := processor.NewDataCombiner(utils.ApiURL{BaseURL: *docStoreAPIBaseURL, Endpoint: *docStoreAPIEndpoint},
			utils.ApiURL{BaseURL: *internalContentAPIBaseURL, Endpoint: *internalContentAPIEndpoint},
			utils.ApiURL{BaseURL: *contentCollectionRWBaseURL, Endpoint: *contentCollectionRWEndpoint}, &client)

		producerConfig := kafka.ProducerConfig{
			BrokersConnectionString: *kafkaAddress,
			Topic:                   *combinedTopic,
			Options:                 kafka.DefaultProducerOptions(),
		}
		messageProducer := kafka.NewProducer(producerConfig, log, 0, time.Minute)

		processorConf := processor.NewMsgProcessorConfig(*whitelistedContentUris, *whitelistedMetadataOriginSystemHeaders, *contentTopic, *metadataTopic)
		msgProcessor := processor.NewMsgProcessor(log, messagesCh, processorConf, dataCombiner, messageProducer, *whitelistedContentTypes)
		go msgProcessor.ProcessMessages()

		// process requested messages - used for re-indexing and forced requests
		forcedProducerConfig := kafka.ProducerConfig{
			BrokersConnectionString: *kafkaAddress,
			Topic:                   *forcedCombinedTopic,
			Options:                 kafka.DefaultProducerOptions(),
		}
		forcedMessageProducer := kafka.NewProducer(forcedProducerConfig, log, 0, time.Minute)

		requestProcessor := processor.NewRequestProcessor(log, dataCombiner, forcedMessageProducer, *whitelistedContentTypes)

		reqHandler := &requestHandler{
			requestProcessor: requestProcessor,
			log:              log,
		}

		// Since the health check for all producers and consumers just checks /topics for a response, we pick a producer and a consumer at random
		healthcheckHandler := NewCombinerHealthcheck(log, messageProducer, mc.Consumer, &client, *docStoreAPIBaseURL, *internalContentAPIBaseURL)

		routeRequests(log, port, reqHandler, healthcheckHandler)
	}

	log.Infof("PostPublicationCombiner is starting with args %v", os.Args)

	err := app.Run(os.Args)
	if err != nil {
		log.WithError(err).Error("App could not start")
	}
}

func routeRequests(log *logger.UPPLogger, port *string, requestHandler *requestHandler, healthService *HealthcheckHandler) {
	r := http.NewServeMux()

	r.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)
	r.HandleFunc(status.PingPath, status.PingHandler)
	r.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(healthService.GTG))

	checks := []health.Check{
		checkKafkaProducerConnectivity(healthService),
		checkKafkaProxyConsumerConnectivity(healthService),
		checkDocumentStoreAPIHealthcheck(healthService),
		checkInternalContentAPIHealthcheck(healthService),
	}

	hc := health.TimedHealthCheck{
		HealthCheck: health.HealthCheck{
			SystemCode:  systemCode,
			Name:        "post-publication-combiner",
			Description: "Checks for service dependencies: document-store, internal-content-api, kafka proxy and the presence of related topics",
			Checks:      checks,
		},
		Timeout: 10 * time.Second,
	}

	r.Handle("/__health", handlers.MethodHandler{"GET": http.HandlerFunc(health.Handler(hc))})

	servicesRouter := mux.NewRouter()
	servicesRouter.HandleFunc("/{id}", requestHandler.postMessage).Methods("POST")

	var monitoringRouter http.Handler = servicesRouter
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(log, monitoringRouter)
	monitoringRouter = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, monitoringRouter)

	r.Handle("/", monitoringRouter)

	server := &http.Server{Addr: ":" + *port, Handler: r}

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Infof("HTTP server closing with message: %v", err)
		}
		wg.Done()
	}()

	waitForSignal()
	log.Infof("[Shutdown] PostPublicationCombiner is shutting down")

	if err := server.Close(); err != nil {
		log.WithError(err).Error("Unable to stop http server")
	}

	wg.Wait()
}

func waitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
