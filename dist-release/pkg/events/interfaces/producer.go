package producer_interfaces

type Producer interface {
	Produce(queueName string, payload []byte, webhookUrl string, userID string) error
	CreateGlobalQueues() error
}
