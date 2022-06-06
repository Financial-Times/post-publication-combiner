package processor

import (
	"net/http"

	"github.com/Financial-Times/message-queue-gonsumer/consumer"
)

type KafkaMessage struct {
	topic   string
	message consumer.Message
}

func NewKafkaConsumer(config consumer.QueueConfig, ch chan<- *KafkaMessage, client *http.Client) consumer.MessageConsumer {
	messageHandler := func(message consumer.Message) {
		ch <- &KafkaMessage{
			topic:   config.Topic,
			message: message,
		}
	}

	return consumer.NewConsumer(config, messageHandler, client)
}
