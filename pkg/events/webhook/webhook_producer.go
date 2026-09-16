package webhook_producer

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	producer_interfaces "github.com/EvolutionAPI/evolution-go/pkg/events/interfaces"
	logger_wrapper "github.com/EvolutionAPI/evolution-go/pkg/logger"
)

const (
	// deliveryTimeout bounds one attempt end to end. Without it a webhook
	// endpoint that accepts the connection and never answers pins a goroutine
	// forever, and the retry below never gets to run.
	deliveryTimeout = 30 * time.Second

	// maxDeliveryAttempts counts the first try plus retries.
	maxDeliveryAttempts = 5

	// retryBaseDelay is doubled each attempt up to retryMaxDelay, so a briefly
	// unavailable endpoint is retried quickly while a long outage backs off.
	retryBaseDelay = 2 * time.Second
	retryMaxDelay  = 60 * time.Second

	// maxResponseBytes caps what is read back. The body is only logged, so a
	// misbehaving endpoint must not be able to stream unbounded data into memory.
	maxResponseBytes = 8 * 1024

	// maxInFlight bounds concurrent deliveries across every instance. A burst of
	// messages must not translate into unbounded goroutines and sockets.
	maxInFlight = 64
)

type webhookProducer struct {
	url           string
	client        *http.Client
	inFlight      chan struct{}
	loggerWrapper *logger_wrapper.LoggerManager

	// Retry knobs, defaulted from the constants above. They are fields rather
	// than plain constants so tests can shrink the waits instead of sleeping
	// through a real backoff.
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
}

func NewWebhookProducer(
	url string,
	loggerWrapper *logger_wrapper.LoggerManager,
) producer_interfaces.Producer {
	transport := (http.DefaultTransport.(*http.Transport)).Clone()
	// Webhooks are usually a handful of endpoints receiving many events, so keep
	// connections warm instead of paying a TLS handshake per delivery.
	transport.MaxIdleConnsPerHost = 16

	return &webhookProducer{
		url:           url,
		client:        &http.Client{Transport: transport, Timeout: deliveryTimeout},
		inFlight:      make(chan struct{}, maxInFlight),
		loggerWrapper: loggerWrapper,
		maxAttempts:   maxDeliveryAttempts,
		baseDelay:     retryBaseDelay,
		maxDelay:      retryMaxDelay,
	}
}

// Produce fans one event out to the global webhook and the instance's own.
//
// Delivery is asynchronous, so a nil return means "accepted for delivery", not
// "delivered" — the outcome is reported through the logs.
func (p *webhookProducer) Produce(
	queueName string,
	payload []byte,
	webhookUrl string,
	userID string,
) error {
	// The queue name is meaningless for HTTP delivery: it names an AMQP queue,
	// and the webhook body already carries the event. It used to be parsed here
	// and anything without a dot was dropped, which silently discarded events
	// posted under a bare name such as "sendstatus".
	if p.url != "" {
		p.deliver(p.url, payload, userID)
	}
	if webhookUrl != "" && webhookUrl != p.url {
		p.deliver(webhookUrl, payload, userID)
	}

	return nil
}

// deliver queues one delivery, dropping it only if the in-flight budget is full
// — which means the endpoints are far behind and queueing more would just grow
// memory until something breaks.
func (p *webhookProducer) deliver(url string, payload []byte, userID string) {
	select {
	case p.inFlight <- struct{}{}:
	default:
		p.loggerWrapper.GetLogger(userID).LogError(
			"[%s] webhook dropped, %d deliveries already in flight - url: %s",
			userID, maxInFlight, url,
		)
		return
	}

	go func() {
		defer func() { <-p.inFlight }()
		p.sendWebhookWithRetry(url, payload, userID)
	}()
}

func (p *webhookProducer) sendWebhookWithRetry(url string, body []byte, userID string) {
	logger := p.loggerWrapper.GetLogger(userID)
	delay := p.baseDelay

	for attempt := 1; attempt <= p.maxAttempts; attempt++ {
		statusCode, responseBody, retryable, err := p.sendWebhook(url, body)
		if err == nil {
			logger.LogInfo(
				"[%s] webhook delivered - url: %s, status: %d, attempt: %d, response: %s",
				userID, url, statusCode, attempt, string(responseBody),
			)
			return
		}

		// 4xx means the endpoint understood the request and refused it. Retrying
		// cannot change that, and five attempts a minute apart only hammer it.
		if !retryable {
			logger.LogError(
				"[%s] webhook rejected, not retrying - url: %s, status: %d, error: %v, response: %s",
				userID, url, statusCode, err, string(responseBody),
			)
			return
		}

		if attempt == p.maxAttempts {
			break
		}

		logger.LogWarn(
			"[%s] webhook failed, retrying in %s - url: %s, attempt: %d/%d, error: %v",
			userID, delay, url, attempt, p.maxAttempts, err,
		)
		time.Sleep(delay)

		if delay < p.maxDelay {
			delay *= 2
			if delay > p.maxDelay {
				delay = p.maxDelay
			}
		}
	}

	logger.LogError("[%s] webhook failed after %d attempts - url: %s", userID, p.maxAttempts, url)
}

// sendWebhook performs one attempt. retryable reports whether trying again could
// plausibly succeed: network failures and 5xx yes, an outright refusal no.
func (p *webhookProducer) sendWebhook(url string, body []byte) (status int, response []byte, retryable bool, err error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		// A malformed URL will not fix itself on the next attempt.
		return 0, nil, false, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "EvolutionGO-Webhook/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, nil, true, err
	}
	defer resp.Body.Close()

	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if readErr != nil {
		return resp.StatusCode, nil, true, fmt.Errorf("failed to read response: %w", readErr)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp.StatusCode, responseBody, false, nil
	}

	// 408 and 429 are the two 4xx that do resolve on their own.
	retryable = resp.StatusCode >= 500 ||
		resp.StatusCode == http.StatusRequestTimeout ||
		resp.StatusCode == http.StatusTooManyRequests

	return resp.StatusCode, responseBody, retryable, fmt.Errorf("received non-2xx response: %s", resp.Status)
}

// CreateGlobalQueues não faz nada para webhook producer
func (p *webhookProducer) CreateGlobalQueues() error {
	return nil
}
