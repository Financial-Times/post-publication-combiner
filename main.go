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
	"github.com/Financial-Times/kafka-client-go/v3"
	"github.com/Financial-Times/post-publication-combiner/v2/processor"
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
	kafkaConsumerGroupID := app.String(cli.StringOpt{
		Name:   "kafkaConsumerGroupID",
		Value:  "content-post-publication-combiner",
		Desc:   "Kafka group id used for message consuming.",
		EnvVar: "KAFKA_CONSUMER_GROUP",
	})
	consumerLagTolerance := app.Int(cli.IntOpt{
		Name:   "consumerLagTolerance",
		Value:  120,
		Desc:   "Kafka lag tolerance",
		EnvVar: "KAFKA_LAG_TOLERANCE",
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
	kafkaAddress := app.String(cli.StringOpt{
		Name:   "kafkaAddress",
		Value:  "kafka:9092",
		Desc:   "Address used to connect to Kafka",
		EnvVar: "KAFKA_ADDR",
	})

	log := logger.NewUPPLogger(serviceName, *logLevel)

	app.Action = func() {
		client := &http.Client{
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
		messagesCh := make(chan *kafka.FTMessage, 100)
		// Please keep in mind that defer function are executed in LIFO order.
		// And deferring the channel must be after deferring the consumers
		defer func() {
			log.Infof("Closing messages channel")
			close(messagesCh)
		}()

		// consume messages from content queue
		consumerConfig := kafka.ConsumerConfig{
			BrokersConnectionString: *kafkaAddress,
			ConsumerGroup:           *kafkaConsumerGroupID,
			ConnectionRetryInterval: time.Minute,
		}
		// make list of topics fot the consumer
		topics := []*kafka.Topic{
			kafka.NewTopic(*contentTopic, kafka.WithLagTolerance(int64(*consumerLagTolerance))),
			kafka.NewTopic(*metadataTopic, kafka.WithLagTolerance(int64(*consumerLagTolerance))),
		}
		consumer := kafka.NewConsumer(consumerConfig, topics, log)
		messageHandler := func(message kafka.FTMessage) {
			messagesCh <- &message
		}
		go consumer.Start(messageHandler)
		defer func(consumer *kafka.Consumer) {
			log.Infof("Closing consumer")
			err := consumer.Close()
			if err != nil {
				log.WithError(err).Error("Consumer could not stop")
			}
		}(consumer)

		// process and forward messages
		docStoreURL := *docStoreAPIBaseURL + *docStoreAPIEndpoint
		internalContentURL := *internalContentAPIBaseURL + *internalContentAPIEndpoint
		contentCollectionURL := *contentCollectionRWBaseURL + *contentCollectionRWEndpoint
		dataCombiner := processor.NewDataCombiner(docStoreURL, internalContentURL, contentCollectionURL, client)

		producerConfig := kafka.ProducerConfig{
			BrokersConnectionString: *kafkaAddress,
			Topic:                   *combinedTopic,
			Options:                 kafka.DefaultProducerOptions(),
			ConnectionRetryInterval: time.Minute,
		}
		messageProducer := kafka.NewProducer(producerConfig, log)
		defer func(messageProducer *kafka.Producer) {
			log.Infof("Closing message producer")
			if err := messageProducer.Close(); err != nil {
				log.WithError(err).Error("Message producer could not stop")
			}
		}(messageProducer)

		processorConf := processor.NewMsgProcessorConfig(
			*whitelistedContentUris,
			*whitelistedMetadataOriginSystemHeaders,
		)
		msgProcessor := processor.NewMsgProcessor(
			log,
			messagesCh,
			processorConf,
			dataCombiner,
			messageProducer,
			*whitelistedContentTypes,
		)
		go msgProcessor.ProcessMessages()

		// process requested messages - used for re-indexing and forced requests
		forcedProducerConfig := kafka.ProducerConfig{
			BrokersConnectionString: *kafkaAddress,
			Topic:                   *forcedCombinedTopic,
			Options:                 kafka.DefaultProducerOptions(),
			ConnectionRetryInterval: time.Minute,
		}
		forcedMessageProducer := kafka.NewProducer(forcedProducerConfig, log)
		defer func(forcedMessageProducer *kafka.Producer) {
			log.Infof("Closing force messages producer")
			if err := forcedMessageProducer.Close(); err != nil {
				log.WithError(err).Error("Force message producer could not stop")
			}
		}(forcedMessageProducer)

		requestProcessor := processor.NewRequestProcessor(dataCombiner, forcedMessageProducer, *whitelistedContentTypes)

		reqHandler := &requestHandler{
			requestProcessor: requestProcessor,
			log:              log,
		}

		// Since the health check for all producers and consumers just checks /topics for a response, we pick a producer and a consumer at random
		healthcheckHandler := NewCombinerHealthcheck(log, messageProducer, consumer, client, *docStoreAPIBaseURL, *internalContentAPIBaseURL)

		routeRequests(log, port, reqHandler, healthcheckHandler)
	}

	log.Infof("PostPublicationCombiner is starting with args %v", os.Args)

	if err := app.Run(os.Args); err != nil {
		log.WithError(err).Error("App could not start")
	}
}

func routeRequests(
	log *logger.UPPLogger,
	port *string,
	requestHandler *requestHandler,
	healthService *HealthcheckHandler,
) {
	r := http.NewServeMux()

	r.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)
	r.HandleFunc(status.PingPath, status.PingHandler)
	r.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(healthService.GTG))

	checks := []health.Check{
		checkKafkaProducerConnectivity(healthService),
		checkKafkaConsumerConnectivity(healthService),
		monitorKafkaConsumers(healthService),
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
	servicesRouter.HandleFunc("/{id}", requestHandler.publishMessage).Methods("POST")

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
