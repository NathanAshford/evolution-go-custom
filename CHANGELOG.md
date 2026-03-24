# Evolution GO - Changelog

## v0.6.0

### 🆕 New Features
- **Version from VERSION file** — Reads version from `VERSION` file at startup instead of hardcoded value

### 🔧 Improvements
- **Makefile compatibility** — Fixed `$(shell)` syntax for GNU Make 3.81 (macOS default)

## v0.5.4

### 🔧 Improvements
- **Update whatsmeow lib**

## v0.5.3

**Docker:** `evoapicloud/evolution-go:0.5.3`

### 🔧 Improvements

- **Update context handling in service methods** 
  - Refactored multiple service methods across various packages to include `context.Background()` as the first argument in client calls. This change ensures that all client interactions are properly context-aware, allowing for better cancellation and timeout management.
  - Updated methods in `call_service.go`, `community_service.go`, `group_service.go`, `message_service.go`, `newsletter_service.go`, `send_service.go`, `user_service.go`, and `whatsmeow.go` to enhance consistency and reliability in handling requests.
  - This adjustment improves the overall robustness of the API by ensuring that all client calls can leverage context for better control over execution flow and resource management.

## v0.5.2

**Docker:** `evoapicloud/evolution-go:0.5.2`

### 🆕 New Features
- **SetProxy Endpoint**: New endpoint `POST /instance/proxy/{instanceId}` to configure proxy for instances
  - Support for proxy with/without authentication
  - Validation of required fields (host, port)
  - Automatic cache update via reconnection
  - Integrated Swagger documentation

### 🔧 Improvements
- **CheckUser Fallback Logic**: Implemented intelligent fallback logic
  - If `formatJid=true` returns `IsInWhatsapp=false`, automatically retries with `formatJid=false`
  - Significant improvement in valid user detection
  - Added `RemoteJID` field to use WhatsApp-validated JID
- **LID/WhatsApp JID Swap**: Automatic handling of special cases
  - When `Sender` comes as `@lid` and `SenderAlt` comes as `@s.whatsapp.net`
  - Automatic inversion: `Sender` and `Chat` receive `@s.whatsapp.net`, `SenderAlt` receives `@lid`
  - Detailed logs for tracking swaps

### 🐛 Bug Fixes
- **SendMessage**: Standardization of WhatsApp-validated `remoteJID` usage
- **User Validation**: Improvement in phone number validation and formatting

---

## v0.5.1

**Docker:** `evoapicloud/evolution-go:0.5.1`

### 🔧 Improvements
- **Instance Deletion**: Enhance instance deletion and media storage path resolution
- **Media Storage**: Improvements in media storage and path resolution

---

## v0.5.0

**Docker:** `evoapicloud/evolution-go:0.5.0`

### 🔧 Improvements
- **Media Storage**: Enhance media storage and logging in Whatsmeow event handling
- **Retry Logic**: Implement retry logic for client connection and message sending
- **Media Handling**: Enhance media handling in event processing

---

## v0.4.9

**Docker:** `evoapicloud/evolution-go:0.4.9`

### 🔧 Improvements
- **Connection Handling**: Add instance update test scenarios and improve connection handling
- **FormatJid Field**: Update FormatJid field to pointer type for better handling in message structures
- **Dependencies**: Update dependencies and fix presence handling in Whatsmeow integration

---

## v0.4.8

**Docker:** `evoapicloud/evolution-go:0.4.8`

### 🔧 Improvements
- **Audio Duration**: Improve audio duration parsing in convertAudioToOpusWithDuration function

---

## v0.4.7

**Docker:** `evoapicloud/evolution-go:0.4.7`

### 🔧 Improvements
- **Phone Number Formatting**: Improve phone number formatting and validation in user service
- **Brazilian/Portuguese Numbers**: Update Brazilian and Portuguese number formatting in utils

### 🆕 New Features
- **Media Handling**: Enhance media handling in event processing

---

## v0.4.6

**Docker:** `evoapicloud/evolution-go:0.4.6`

### 🆕 New Features
- **User Existence Check**: Add user existence check configuration and JID validation middleware

---

## v0.4.5

**Docker:** `evoapicloud/evolution-go:0.4.5`

### 🔧 Improvements
- **Dependencies**: Update dependencies and enhance audio conversion functionality

---

## v0.4.4

**Docker:** `evoapicloud/evolution-go:0.4.4`

### 🆕 New Features
- **CLAUDE.md**: Add CLAUDE.md for project documentation and enhance RabbitMQ connection handling

---

## v0.4.3

**Docker:** `evoapicloud/evolution-go:0.4.3`

### 🔧 Improvements
- **PostgreSQL Connection**: Fix in PostgreSQL connection configuration for session auth
  - Controlled configuration of pool, idle, etc.
  - Adjustment on top of whatsmeow lib
- **User Endpoints**: Fix in 'User Info' and 'Check User' endpoints
  - Now return with contact's LID information

---

## v0.3.0

### 🆕 New Features
- **Own Message Reactions**: Additional 'fromMe' parameter using Chat id
- **CreatedAt Field**: CreatedAt field added to instances table

---

## v0.2.0

### 🆕 New Features
- **Advanced Settings**: Advanced configurations in instance creation
  - `alwaysOnline` (still to be implemented)
  - `rejectCall` - Automatically reject calls
  - `msgRejectCall` - Call rejection message
  - `readMessages` - Automatically mark messages as read
  - `ignoreGroups` - Ignore group messages
  - `ignoreStatus` - Ignore status messages
- **Advanced Settings Routes**: New routes for get and update of advanced settings
- **QR Code Control**: `QRCODE_MAX_COUNT` variable to control how many QR codes to generate before timeout
- **AMQP Events**: `AMQP_SPECIFIC_EVENTS` variable to individually select which events to receive in RabbitMQ

### 🔧 Improvements
- **Reconnect Endpoint**: Fix in reconnect endpoint
- **Sender Info**: `Sender` and `SenderAlt` no longer come with session id, only the id

### 🐛 Bug Fixes
- **QR Code Generation**: Fix to not generate QR code automatically after disconnection or logout

---

## v0.1.0

### 🆕 Initial Features
- Base implementation of Evolution API in Go
- WhatsApp integration via whatsmeow
- Instance system
- Basic message sending endpoints
- Webhook support
- RabbitMQ and NATS integration
- Authentication system
- Swagger documentation

---

## 📋 Migration Notes

### v0.5.2
- The new `SetProxy` endpoint requires admin permissions (`AuthAdmin`)
- The `CheckUser` fallback logic is automatic and transparent
- LID/WhatsApp JID handling is automatic

### v0.4.3
- Check PostgreSQL connection settings if using postgres auth

### v0.2.0
- Review advanced settings configurations if necessary
- Configure `QRCODE_MAX_COUNT` if you want to limit QR codes
- Configure `AMQP_SPECIFIC_EVENTS` for specific RabbitMQ events

---

## 🔗 Useful Links

- **Docker Hub**: `evoapicloud/evolution-go`
- **Documentation**: Swagger available at `/swagger/`
- **GitHub**: [Evolution API Go](https://github.com/EvolutionAPI/evolution-go)

---

## 🤝 Contributing

To contribute to the project:
1. Fork the repository
2. Create a branch for your feature
3. Commit your changes
4. Open a Pull Request

---

*Last updated: October 2025*

