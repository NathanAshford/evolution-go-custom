// Package registry holds the live WhatsApp clients, one per instance.
//
// It exists because that state is genuinely shared: the HTTP handlers, the
// whatsmeow event goroutines and the auto-reconnect loop all reach for the same
// client at the same time. It used to be a bare map[string]*whatsmeow.Client
// passed by value into a dozen services, written from every one of them with no
// synchronisation at all — which is a data race, and Go answers a concurrent map
// write with `fatal error: concurrent map writes`, killing the whole process
// rather than the goroutine that did it.
//
// Every access goes through the mutex here, so a client can be started, used and
// torn down from different goroutines without that risk.
package registry

import (
	"sync"

	"go.mau.fi/whatsmeow"
)

// Entry is one instance's live client plus the signal that stops it.
//
// The stop signal is a channel that gets CLOSED rather than sent to. That
// distinction matters: the old code sent `true` on an unbuffered channel, so
// signalling blocked until someone happened to be receiving, and a second
// signal — or a signal after a close — panicked the process. A close is
// non-blocking, wakes every waiter at once, and sync.Once makes it safe to call
// as many times as the shutdown paths happen to call it.
type Entry struct {
	mu     sync.RWMutex
	client *whatsmeow.Client

	done     chan struct{}
	stopOnce sync.Once
}

// Client returns the whatsmeow client, or nil while one is still being built.
func (e *Entry) Client() *whatsmeow.Client {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.client
}

// setClient attaches the client once whatsmeow has built it.
func (e *Entry) setClient(client *whatsmeow.Client) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.client = client
}

// Done is closed when this entry is stopped. Select on it to end a goroutine
// that belongs to the instance.
func (e *Entry) Done() <-chan struct{} { return e.done }

// Stopped reports whether this entry has been superseded or torn down.
func (e *Entry) Stopped() bool {
	select {
	case <-e.done:
		return true
	default:
		return false
	}
}

// Stop signals every goroutine watching this entry. Safe to call repeatedly and
// from any goroutine; it never blocks.
func (e *Entry) Stop() {
	e.stopOnce.Do(func() { close(e.done) })
}

// Clients is the process-wide set of live instances.
type Clients struct {
	mu      sync.RWMutex
	entries map[string]*Entry
}

func New() *Clients {
	return &Clients{entries: make(map[string]*Entry)}
}

// Begin opens a fresh entry for an instance and returns it.
//
// Any previous entry is stopped first. That is the whole point: starting an
// instance that was already running used to overwrite the map slot and leave the
// old goroutine watching a channel nobody would ever signal again, so it lived
// until the process died — one leaked goroutine, and one live WhatsApp socket,
// per reconnect.
func (c *Clients) Begin(instanceID string) *Entry {
	c.mu.Lock()
	defer c.mu.Unlock()

	if previous, ok := c.entries[instanceID]; ok {
		previous.Stop()
	}

	entry := &Entry{done: make(chan struct{})}
	c.entries[instanceID] = entry
	return entry
}

// Entry returns the current entry for an instance, if it has one.
func (c *Clients) Entry(instanceID string) (*Entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[instanceID]
	return entry, ok
}

// Get returns an instance's client, or nil when it has none — the same shape the
// old map lookup had, so callers can keep their `if client == nil` checks.
func (c *Clients) Get(instanceID string) *whatsmeow.Client {
	entry, ok := c.Entry(instanceID)
	if !ok {
		return nil
	}
	return entry.Client()
}

// Attach publishes a client on the entry that claimed the instance, but only
// while that entry is still the current one.
//
// The ownership check matters: a slow start whose entry was superseded mid-way
// would otherwise publish its client into the newer entry, so callers would
// hand work to a socket that is about to be torn down while the client that
// actually owns the instance stayed invisible. Reports whether it was published.
func (c *Clients) Attach(instanceID string, entry *Entry, client *whatsmeow.Client) bool {
	c.mu.RLock()
	current := c.entries[instanceID]
	c.mu.RUnlock()

	if current != entry || entry.Stopped() {
		return false
	}

	entry.setClient(client)
	return true
}

// Stop signals an instance's goroutines but keeps the entry in place. Reports
// whether there was anything to signal.
func (c *Clients) Stop(instanceID string) bool {
	entry, ok := c.Entry(instanceID)
	if !ok {
		return false
	}
	entry.Stop()
	return true
}

// Remove stops an instance and forgets it. Reports whether it was present.
func (c *Clients) Remove(instanceID string) bool {
	c.mu.Lock()
	entry, ok := c.entries[instanceID]
	delete(c.entries, instanceID)
	c.mu.Unlock()

	if ok {
		entry.Stop()
	}
	return ok
}

// RemoveOwned forgets an instance unless a DIFFERENT entry has taken it over.
// It reports whether the caller should clean up the rest of the instance's
// state.
//
// A goroutine that has been superseded must clean up after ITSELF and leave the
// newer one alone: removing the instance unconditionally would tear down the
// client that had just replaced it, so the instance would look started while
// nothing was serving it.
//
// An instance whose slot is already gone — Remove ran first, which is the usual
// case for a deliberate stop — still returns true: nothing newer owns it, so
// the caller is the one responsible for the leftovers.
func (c *Clients) RemoveOwned(instanceID string, entry *Entry) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	current, ok := c.entries[instanceID]
	if ok && current != entry {
		return false
	}
	if ok {
		delete(c.entries, instanceID)
	}
	entry.Stop()
	return true
}

// Has reports whether an instance currently holds a client.
func (c *Clients) Has(instanceID string) bool {
	return c.Get(instanceID) != nil
}

// IDs lists the instances that currently have an entry.
func (c *Clients) IDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := make([]string, 0, len(c.entries))
	for id := range c.entries {
		ids = append(ids, id)
	}
	return ids
}
