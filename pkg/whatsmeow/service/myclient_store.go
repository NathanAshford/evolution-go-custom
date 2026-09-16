package whatsmeow_service

import (
	"sync"

	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
)

// myClientStore holds the per-instance event-handler state.
//
// Like the client registry, this replaces a bare map that was written from HTTP
// handlers and read from whatsmeow's event goroutines with no lock — a data
// race that Go punishes with a process-wide fatal error rather than a recovered
// panic.
type myClientStore struct {
	mu      sync.RWMutex
	clients map[string]*MyClient
}

func newMyClientStore() *myClientStore {
	return &myClientStore{clients: make(map[string]*MyClient)}
}

func (s *myClientStore) get(instanceID string) (*MyClient, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	client, ok := s.clients[instanceID]
	return client, ok
}

func (s *myClientStore) set(instanceID string, client *MyClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[instanceID] = client
}

func (s *myClientStore) remove(instanceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, instanceID)
}

// Instance returns the instance this client was built for.
//
// The settings endpoints swap this pointer while the event goroutine is using
// it, so the read has to be synchronised — otherwise a settings update could be
// observed half-applied, or not at all.
func (mycli *MyClient) Instance() *instance_model.Instance {
	mycli.mu.RLock()
	defer mycli.mu.RUnlock()
	return mycli.instance
}

// setInstance swaps in a freshly loaded instance.
func (mycli *MyClient) setInstance(instance *instance_model.Instance) {
	mycli.mu.Lock()
	defer mycli.mu.Unlock()
	mycli.instance = instance
}

// Runtime event/transport toggles, read by the event goroutine and rewritten by
// the settings endpoints — same reasoning as Instance above.
func (mycli *MyClient) setRuntimeTargets(webhookUrl, rabbitmq, nats, websocket string) {
	mycli.mu.Lock()
	defer mycli.mu.Unlock()
	mycli.webhookUrl = webhookUrl
	mycli.rabbitmqEnable = rabbitmq
	mycli.natsEnable = nats
	mycli.websocketEnable = websocket
}

func (mycli *MyClient) setSubscriptions(subscriptions []string) {
	mycli.mu.Lock()
	defer mycli.mu.Unlock()
	mycli.subscriptions = subscriptions
}

func (mycli *MyClient) webhookTarget() string {
	mycli.mu.RLock()
	defer mycli.mu.RUnlock()
	return mycli.webhookUrl
}

func (mycli *MyClient) transportFlags() (rabbitmq, nats, websocket string) {
	mycli.mu.RLock()
	defer mycli.mu.RUnlock()
	return mycli.rabbitmqEnable, mycli.natsEnable, mycli.websocketEnable
}

func (mycli *MyClient) subscribedEvents() []string {
	mycli.mu.RLock()
	defer mycli.mu.RUnlock()
	return mycli.subscriptions
}
