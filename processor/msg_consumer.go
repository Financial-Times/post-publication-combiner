package processor

import (
	"net/http"

	logger "github.com/Financial-Times/go-logger/v2"
	consumer "github.com/Financial-Times/message-queue-gonsumer"
)

type QConsumer interface {
	ProcessMsg(m consumer.Message)
}

type KafkaQConsumer struct {
	Consumer consumer.MessageConsumer
	dest     chan<- *KafkaQMessage
	msgType  string
}

type KafkaQMessage struct {
	msgType string
	msg     consumer.Message
}

func NewKafkaQConsumer(cConf consumer.QueueConfig, ch chan<- *KafkaQMessage, client *http.Client, l *logger.UPPLogger) *KafkaQConsumer {
	kc := KafkaQConsumer{msgType: cConf.Topic, dest: ch}
	kc.Consumer = consumer.NewConsumer(cConf, kc.ProcessMsg, client, l)
	return &kc
}

func (c *KafkaQConsumer) ProcessMsg(m consumer.Message) {
	c.dest <- &KafkaQMessage{msgType: c.msgType, msg: m}
}
