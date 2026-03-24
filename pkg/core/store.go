package core

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"strconv"

	"gorm.io/gorm"
)

// db reference set by SetDB — used by all store operations.
var globalDB *gorm.DB

// SetDB sets the database connection for the core package.
// Must be called before InitializeRuntime.
func SetDB(db *gorm.DB) {
	globalDB = db
}

// MigrateDB runs auto-migration for runtime_configs table.
func MigrateDB() error {
	if globalDB == nil {
		return fmt.Errorf("core: database not set, call SetDB first")
	}
	return globalDB.AutoMigrate(&RuntimeConfig{})
}

// getConfig reads a config value from the database.
func getConfig(key string) (string, error) {
	if globalDB == nil {
		return "", fmt.Errorf("core: database not set")
	}
	var cfg RuntimeConfig
	result := globalDB.Where("key = ?", key).First(&cfg)
	if result.Error != nil {
		return "", result.Error
	}
	return cfg.Value, nil
}

// setConfig writes a config value to the database (upsert).
func setConfig(key, value string) error {
	if globalDB == nil {
		return fmt.Errorf("core: database not set")
	}
	var cfg RuntimeConfig
	result := globalDB.Where("key = ?", key).First(&cfg)
	if result.Error != nil {
		// Create
		return globalDB.Create(&RuntimeConfig{Key: key, Value: value}).Error
	}
	// Update
	return globalDB.Model(&cfg).Update("value", value).Error
}

// deleteConfig removes a config key from the database.
func deleteConfig(key string) {
	if globalDB == nil {
		return
	}
	globalDB.Where("key = ?", key).Delete(&RuntimeConfig{})
}

// RuntimeData persisted in database after registration.
type RuntimeData struct {
	APIKey     string
	Tier       string
	CustomerID int
}

// loadRuntimeData reads the saved license from database.
func loadRuntimeData() (*RuntimeData, error) {
	apiKey, err := getConfig(ConfigKeyAPIKey)
	if err != nil || apiKey == "" {
		return nil, fmt.Errorf("no license found")
	}

	tier, _ := getConfig(ConfigKeyTier)
	customerIDStr, _ := getConfig(ConfigKeyCustomerID)
	customerID, _ := strconv.Atoi(customerIDStr)

	return &RuntimeData{
		APIKey:     apiKey,
		Tier:       tier,
		CustomerID: customerID,
	}, nil
}

// saveRuntimeData writes the license to database.
func saveRuntimeData(rd *RuntimeData) error {
	if err := setConfig(ConfigKeyAPIKey, rd.APIKey); err != nil {
		return err
	}
	if err := setConfig(ConfigKeyTier, rd.Tier); err != nil {
		return err
	}
	if rd.CustomerID > 0 {
		if err := setConfig(ConfigKeyCustomerID, strconv.Itoa(rd.CustomerID)); err != nil {
			return err
		}
	}
	return nil
}

// removeRuntimeData deletes the license from database.
func removeRuntimeData() {
	deleteConfig(ConfigKeyAPIKey)
	deleteConfig(ConfigKeyTier)
	deleteConfig(ConfigKeyCustomerID)
}

// loadOrCreateInstanceID generates or loads a persistent instance ID from database.
func loadOrCreateInstanceID() (string, error) {
	id, err := getConfig(ConfigKeyInstanceID)
	if err == nil && len(id) == 36 {
		return id, nil
	}

	// Generate hardware-based instance ID
	id = generateHardwareID()
	if id == "" {
		// Fallback to random UUID
		id, err = newUUID()
		if err != nil {
			return "", err
		}
	}

	if err := setConfig(ConfigKeyInstanceID, id); err != nil {
		return "", err
	}
	return id, nil
}

// generateHardwareID creates a deterministic ID from MAC + hostname.
func generateHardwareID() string {
	hostname, _ := os.Hostname()
	macAddr := getPrimaryMAC()
	if hostname == "" && macAddr == "" {
		return ""
	}

	seed := hostname + "|" + macAddr
	h := make([]byte, 16)
	copy(h, []byte(seed))
	for i := 16; i < len(seed); i++ {
		h[i%16] ^= seed[i]
	}
	h[6] = (h[6] & 0x0f) | 0x40 // version 4
	h[8] = (h[8] & 0x3f) | 0x80 // variant
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

func getPrimaryMAC() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		if len(iface.HardwareAddr) > 0 {
			return iface.HardwareAddr.String()
		}
	}
	return ""
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
