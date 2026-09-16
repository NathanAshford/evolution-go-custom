package whatsmeow_service

import (
	"strings"
	"sync"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/store"
	"google.golang.org/protobuf/proto"
)

// store.DeviceProps is a package-level pointer in whatsmeow: ONE struct for the
// whole process. whatsmeow reads it while building the pairing payload, deep
// inside Connect's handshake — there is no per-client override.
//
// This process runs many instances, each with its own OS name and version, and
// each used to write those fields straight into that shared struct just before
// connecting. Two instances starting at once therefore raced: the second
// overwrote the first's fields between the first's write and its handshake, so
// an instance could pair advertising another instance's device — and concurrent
// writes to the same map/pointer fields are a data race, which in Go can take
// the whole process down rather than just the goroutine.
//
// deviceProps serialises the write-then-connect region. Connect is synchronous
// through the handshake, so the critical section is bounded by one dial and one
// handshake, not by the QR wait that follows.
var devicePropsMu sync.Mutex

// applyDeviceProps writes the pairing identity for one instance into the shared
// struct. Callers must hold devicePropsMu.
//
// DeviceProps describes the companion device in the PAIRING payload only, so
// this is for a QR/passkey login. The handshake version is separate and applies
// to every connection — see applyWAVersion.
func applyDeviceProps(osName string, version clientVersion) {
	if platformID, ok := waCompanionReg.DeviceProps_PlatformType_value[strings.ToUpper("chrome")]; ok {
		store.DeviceProps.PlatformType = waCompanionReg.DeviceProps_PlatformType(platformID).Enum()
	}

	// proto.String copies the value. Assigning &cd.Instance.OsName instead
	// aliased a field of the instance row into the global, so a later write to
	// that row silently changed what every subsequent pairing advertised.
	store.DeviceProps.Os = proto.String(osName)
	store.DeviceProps.RequireFullSync = proto.Bool(true)

	if version.Major == 0 && version.Minor == 0 && version.Patch == 0 {
		return
	}

	store.DeviceProps.Version = &waCompanionReg.DeviceProps_AppVersion{
		Primary:   proto.Uint32(uint32(version.Major)),
		Secondary: proto.Uint32(uint32(version.Minor)),
		Tertiary:  proto.Uint32(uint32(version.Patch)),
	}
}

// connectWithDeviceProps applies this instance's pairing identity and connects
// with it, holding the lock across both so no other instance can overwrite the
// props in between.
func connectWithDeviceProps(client *whatsmeow.Client, osName string, version clientVersion) error {
	devicePropsMu.Lock()
	defer devicePropsMu.Unlock()

	applyDeviceProps(osName, version)
	applyWAVersion(version)
	return client.Connect()
}

// connectPaired connects an already-paired client. It keeps the DeviceProps it
// paired with — those only describe the pairing — but the handshake version
// still has to be set: whatsmeow ships a hardcoded one that goes stale, and
// WhatsApp answers 405 "client outdated" to a connection that advertises it.
//
// The lock is held across both so the handshake cannot read the globals while
// another instance is mid-write.
func connectPaired(client *whatsmeow.Client, version clientVersion) error {
	devicePropsMu.Lock()
	defer devicePropsMu.Unlock()

	applyWAVersion(version)
	return client.Connect()
}

// connectShared connects a client with whatever globals are currently set — for
// a reconnect of a client that is already running, where the version was
// established when it first connected. It still takes the lock so the handshake
// cannot race another instance's write.
func connectShared(client *whatsmeow.Client) error {
	devicePropsMu.Lock()
	defer devicePropsMu.Unlock()

	return client.Connect()
}
