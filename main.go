package main

import (
	"github.com/Financial-Times/base-ft-rw-app-go/baseftrwapp"
	health "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/http-handlers-go/httphandlers"
	"github.com/Financial-Times/message-queue-gonsumer/consumer"
	"github.com/Financial-Times/post-publication-combiner/processor"
	"github.com/Financial-Times/post-publication-combiner/utils"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jawher/mow.cli"
	"github.com/rcrowley/go-metrics"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {

	app := cli.App("post-publication-combiner", "Service listening to content and metadata PostPublication events, and forwards a combined message to the queue")

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
		Value:  "PostMetadataPublicationEvents",
		EnvVar: "KAFKA_METADATA_TOPIC_NAME",
	})
	combinedTopic := app.String(cli.StringOpt{
		Name:   "combinedTopic",
		Value:  "CombinedPostPublicationEvents",
		EnvVar: "KAFKA_COMBINED_TOPIC_NAME",
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

	graphiteTCPAddress := app.String(cli.StringOpt{
		Name:   "graphiteTCPAddress",
		Desc:   "Graphite TCP address, e.g. graphite.ft.com:2003. Leave as default if you do NOT want to output to graphite (e.g. if running locally",
		EnvVar: "GRAPHITE_ADDRESS",
	})
	graphitePrefix := app.String(cli.StringOpt{
		Name:   "graphitePrefix",
		Desc:   "Prefix to use. Should start with content, include the environment, and the host name. e.g. coco.pre-prod.service-name.1 or content.test.people.rw.service-name.ftaps58938-law1a-eu-t",
		EnvVar: "GRAPHITE_PREFIX",
	})
	logMetrics := app.Bool(cli.BoolOpt{
		Name:   "logMetrics",
		Value:  false,
		Desc:   "Whether to log metrics. Set to true if running locally and you want metrics output",
		EnvVar: "LOG_METRICS",
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
		Value:  "/content/{uuid}/annotations/{platformVersion}",
		Desc:   "The endpoint used for metadata retrieval.",
		EnvVar: "PUBLIC_ANNOTATIONS_API_ENDPOINT",
	})
	whitelistedMetadataOriginSystemHeaders := app.Strings(cli.StringsOpt{
		Name:   "whitelistedMetadataOriginSystemHeaders",
		Value:  []string{"http://cmdb.ft.com/systems/binding-service", "http://cmdb.ft.com/systems/methode-web-pub", "http://cmdb.ft.com/systems/next-video-editor"},
		Desc:   "Origin-System-Ids that are supported to be processed from the PostPublicationEvents queue.",
		EnvVar: "WHITELISTED_METADATA_ORIGIN_SYSTEM_HEADERS",
	})
	whitelistedContentUris := app.Strings(cli.StringsOpt{
		Name:   "whitelistedContentURIs",
		Value:  []string{"methode-article-mapper", "wordpress-article-mapper", "next-video-mapper"},
		Desc:   "Space separated list with content URI substrings - to identify accepted content types.",
		EnvVar: "WHITELISTED_CONTENT_URI",
	})

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

		baseftrwapp.OutputMetricsIfRequired(*graphiteTCPAddress, *graphitePrefix, *logMetrics)

		// create channel for holding the post publication content and metadata messages
		messagesCh := make(chan *processor.KafkaQMessage, 100)

		// consumer messages from content queue
		cConf := consumer.QueueConfig{
			Addrs: []string{*kafkaProxyAddress},
			Group: *kafkaContentConsumerGroup,
			Topic: *contentTopic,
			Queue: *kafkaProxyRoutingHeader,
		}
		cc := processor.NewKafkaQConsumer(cConf, messagesCh, &client)
		go cc.Consumer.Start()
		defer cc.Consumer.Stop()

		// consumer messages from metadata queue
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
		pQConf := processor.NewProducerConfig(*kafkaProxyAddress, *combinedTopic, *kafkaProxyRoutingHeader)
		processorConf := processor.NewMsgProcessorConfig(
			*whitelistedContentUris,
			*whitelistedMetadataOriginSystemHeaders,
			*contentTopic,
			*metadataTopic,
		)
		msgProcessor := processor.NewMsgProcessor(
			pQConf,
			messagesCh,
			utils.ApiURL{*docStoreAPIBaseURL, *docStoreAPIEndpoint},
			utils.ApiURL{*publicAnnotationsAPIBaseURL, *publicAnnotationsAPIEndpoint},
			&client,
			processorConf)
		go msgProcessor.ProcessMessages()

		routeRequests(port, &requestHandler{processor: msgProcessor}, NewCombinerHealthcheck(*kafkaProxyAddress, *kafkaProxyRoutingHeader, &client, *contentTopic, *metadataTopic, *combinedTopic, *docStoreAPIBaseURL, *publicAnnotationsAPIBaseURL))
	}

	logrus.SetLevel(logrus.InfoLevel)
	logrus.Infof("PostPublicationCombiner is starting with args %v", os.Args)

	err := app.Run(os.Args)
	if err != nil {
		logrus.Errorf("App could not start, error=[%v]\n", err)
	}
}

func routeRequests(port *string, requestHandler *requestHandler, healthService *HealthcheckHandler) {

	r := http.NewServeMux()

	r.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)
	r.HandleFunc(status.PingPath, status.PingHandler)
	r.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(healthService.gtgCheck))

	checks := []health.Check{
		checkPostMetadataPublicationFoundHealthcheck(healthService),
		checkPostContentPublicationTopicIsFoundHealthcheck(healthService),
		checkCombinedPublicationTopicTopicIsFoundHealthcheck(healthService),
		checkDocumentStoreAPIHealthcheck(healthService),
		checkPublicAnnotationsAPIHealthcheck(healthService),
	}

	hc := health.HealthCheck{
		SystemCode:  "upp-post-publication-combiner",
		Name:        "post-publication-combiner",
		Description: "Checks for service dependencies: document-store, public-annotations-api, kafka proxy and the presence of related topics",
		Checks:      checks,
	}

	r.Handle("/__health", handlers.MethodHandler{"GET": http.HandlerFunc(health.Handler(hc))})

	servicesRouter := mux.NewRouter()
	servicesRouter.HandleFunc("/{content-type}/{id}", requestHandler.postMessage).Methods("POST")

	var monitoringRouter http.Handler = servicesRouter
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(logrus.StandardLogger(), monitoringRouter)
	monitoringRouter = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, monitoringRouter)

	r.Handle("/", monitoringRouter)

	server := &http.Server{Addr: ":" + *port, Handler: r}

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			logrus.Infof("HTTP server closing with message: %v", err)
		}
		wg.Done()
	}()

	waitForSignal()
	logrus.Infof("[Shutdown] PostPublicationCombiner is shutting down")

	if err := server.Close(); err != nil {
		logrus.Errorf("Unable to stop http server: %v", err)
	}

	wg.Wait()

}

func waitForSignal() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
