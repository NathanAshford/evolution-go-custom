package instance_handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	config "github.com/EvolutionAPI/evolution-go/pkg/config"
	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	instance_service "github.com/EvolutionAPI/evolution-go/pkg/instance/service"
	"github.com/EvolutionAPI/evolution-go/pkg/utils"
)

type InstanceHandler interface {
	Create(ctx *gin.Context)
	Connect(ctx *gin.Context)
	Reconnect(ctx *gin.Context)
	Disconnect(ctx *gin.Context)
	Logout(ctx *gin.Context)
	Delete(ctx *gin.Context)
	Status(ctx *gin.Context)
	Limits(ctx *gin.Context)
	Qr(ctx *gin.Context)
	All(ctx *gin.Context)
	Info(ctx *gin.Context)
	Pair(ctx *gin.Context)
	SetProxy(ctx *gin.Context)
	GetProxy(ctx *gin.Context)
	ReconnectProxy(ctx *gin.Context)
	TestProxy(ctx *gin.Context)
	DeleteProxy(ctx *gin.Context)
	ForceReconnect(ctx *gin.Context)
	GetLogs(ctx *gin.Context)
	GetAdvancedSettings(ctx *gin.Context)
	UpdateAdvancedSettings(ctx *gin.Context)
	AddWebhook(ctx *gin.Context)
	RemoveWebhook(ctx *gin.Context)
	ListWebhooks(ctx *gin.Context)
}

type instanceHandler struct {
	config          *config.Config
	instanceService instance_service.InstanceService
}

// Create a new instance
// @Summary Create a new instance
// @Description Creates a new instance with the provided data including optional advanced settings
// @Tags Instance
// @Accept json
// @Produce json
// @Param instance body instance_service.CreateStruct true "Instance data with optional advanced settings"
// @Success 200 {object} gin.H "Instance created successfully"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/create [post]
func (i *instanceHandler) Create(ctx *gin.Context) {
	var data *instance_service.CreateStruct
	err := ctx.ShouldBindJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	if data.Token == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	if data.Proxy != nil {
		if data.Proxy.Port == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "proxy port is required"})
			return
		}

		if data.Proxy.Password == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "proxy password is required"})
			return
		}

		if data.Proxy.Username == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "proxy username is required"})
			return
		}

		if data.Proxy.Host == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "proxy host is required"})
			return
		}
	} else {
		if i.config.ProxyHost != "" && i.config.ProxyPort != "" && i.config.ProxyUsername != "" && i.config.ProxyPassword != "" {
			data.Proxy = &instance_service.ProxyConfig{
				Host:     i.config.ProxyHost,
				Port:     i.config.ProxyPort,
				Username: i.config.ProxyUsername,
				Password: i.config.ProxyPassword,
			}
		}
	}

	createdInstance, err := i.instanceService.Create(data)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": createdInstance})
}

// Connect to instance
// @Summary Connect to instance
// @Description Connect to instance with the provided data
// @Tags Instance
// @Accept json
// @Produce json
// @Param instance body instance_service.ConnectStruct true "Instance data"
// @Success 200 {object} gin.H "Instance connected successfully"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/connect [post]
func (i *instanceHandler) Connect(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *instance_service.ConnectStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	instance, jid, eventString, err := i.instanceService.Connect(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.Set("instance", instance)

	responseData := gin.H{
		"jid":         jid,
		"webhookUrl":  instance.Webhook,
		"eventString": eventString,
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": responseData})
}

// Reconnect to instance
// @Summary Reconnect to instance
// @Description Reconnect to instance
// @Tags Instance
// @Accept json
// @Produce json
// @Success 200 {object} gin.H "Instance reconnected successfully"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/reconnect [post]
func (i *instanceHandler) Reconnect(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	err := i.instanceService.Reconnect(instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Disconnect from instance
// @Summary Disconnect from instance
// @Description Disconnect from instance
// @Tags Instance
// @Accept json
// @Produce json
// @Success 200 {object} gin.H "Instance disconnected successfully"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/disconnect [post]
func (i *instanceHandler) Disconnect(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	updateInstance, err := i.instanceService.Disconnect(instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.Set("instance", updateInstance)

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Logout from instance
// @Summary Logout from instance
// @Description Logout from instance
// @Tags Instance
// @Accept json
// @Produce json
// @Success 200 {object} gin.H "Instance logged out successfully"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/logout [delete]
func (i *instanceHandler) Logout(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}

	updateInstance, err := i.instanceService.Logout(instance)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.Set("instance", updateInstance)

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Get instance status
// @Summary Get instance status
// @Description Get instance status
// @Tags Instance
// @Accept json
// @Produce json
// @Success 200 {object} gin.H "Instance status"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/status [get]
func (i *instanceHandler) Status(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}

	status, err := i.instanceService.Status(instance)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": status})
}

// Get instance QR code
// @Summary Get instance QR code
// @Description Get instance QR code
// @Tags Instance
// @Accept json
// @Produce json
// @Success 200 {object} gin.H "Instance QR code"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/qr [get]
func (i *instanceHandler) Qr(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}

	qrcode, err := i.instanceService.GetQr(instance)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": qrcode})
}

// Request pairing code
// @Summary Request pairing code
// @Description Request pairing code
// @Tags Instance
// @Accept json
// @Produce json
// @Param instance body instance_service.PairStruct true "Instance data"
// @Success 200 {object} gin.H "Pairing code"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/pair [post]
func (i *instanceHandler) Pair(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *instance_service.PairStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Phone == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "phone is required"})
		return
	}

	pairingCode, err := i.instanceService.Pair(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": pairingCode})
}

// Get all instances
// @Summary Get all instances
// @Description Get all instances
// @Tags Instance
// @Accept json
// @Produce json
// @Success 200 {object} gin.H "All instances"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/all [get]
func (i *instanceHandler) All(ctx *gin.Context) {
	instances, err := i.instanceService.GetAll()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": instances})
}

// Get instance
// @Summary Get instance
// @Description Get instance
// @Tags Instance
// @Accept json
// @Produce json
// @Param instanceId path string true "Instance Id"
// @Success 200 {object} gin.H "Instance"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/info/{instanceId} [get]
func (i *instanceHandler) Info(ctx *gin.Context) {
	instanceId := ctx.Param("instanceId")

	if instanceId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	instance, err := i.instanceService.Info(instanceId)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": instance})
}

// Limits returns WhatsApp's account-level messaging limits (reachout timelock + new-chat
// quota) for an instance. Used by the UI to show a countdown until a 463 timelock ends.
// @Summary Get account messaging limits
// @Description Returns the reachout timelock and new-chat capping info for the instance
// @Tags Instance
// @Produce json
// @Param instanceId path string true "Instance ID"
// @Success 200 {object} gin.H "Limits"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/limits/{instanceId} [get]
func (i *instanceHandler) Limits(ctx *gin.Context) {
	instanceId := ctx.Param("instanceId")

	if instanceId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	limits, err := i.instanceService.GetLimits(instanceId)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": limits})
}

// Delete instance
// @Summary Delete instance
// @Description Delete instance
// @Tags Instance
// @Accept json
// @Produce json
// @Param instanceId path string true "Instance Id"
// @Success 200 {object} gin.H "Instance deleted successfully"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/delete/{instanceId} [delete]
func (i *instanceHandler) Delete(ctx *gin.Context) {
	instanceId := ctx.Param("instanceId")

	if instanceId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	err := i.instanceService.Delete(instanceId)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Set proxy
// @Summary Set proxy configuration
// @Description Set proxy configuration for an instance
// @Tags Instance
// @Accept json
// @Produce json
// @Param instanceId path string true "Instance id"
// @Param proxy body instance_service.SetProxyStruct true "Proxy configuration"
// @Success 200 {object} gin.H "Proxy set successfully"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/proxy/{instanceId} [post]
func (i *instanceHandler) SetProxy(ctx *gin.Context) {
	instanceId := ctx.Param("instanceId")

	if instanceId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	var data *instance_service.SetProxyStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if data.Host == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "host is required"})
		return
	}

	if data.Port == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "port is required"})
		return
	}

	err = i.instanceService.SetProxyFromStruct(instanceId, data)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responseData := gin.H{
		"protocol": utils.NormalizeProxyProtocol(data.Protocol, data.Port),
		"host":     data.Host,
		"port":     data.Port,
		"hasAuth":  data.Username != "" && data.Password != "",
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": responseData})
}

// Delete proxy
// @Summary Delete proxy
// @Description Delete proxy
// @Tags Instance
// @Accept json
// @Produce json
// @Param instanceId path string true "Instance id"
// @Success 200 {object} gin.H "Proxy deleted successfully"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/proxy/{instanceId} [delete]
// Get proxy
// @Summary Get proxy configuration
// @Description Returns the proxy configuration saved for an instance (null when none is set)
// @Tags Instance
// @Accept json
// @Produce json
// @Param instanceId path string true "Instance ID"
// @Success 200 {object} gin.H "Proxy configuration"
// @Failure 400 {object} gin.H "instanceId is required"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /instance/proxy/{instanceId} [get]
func (i *instanceHandler) GetProxy(ctx *gin.Context) {
	instanceId := ctx.Param("instanceId")

	if instanceId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	proxyConfig, err := i.instanceService.GetProxy(instanceId)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// data is null when the instance has no proxy configured.
	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": proxyConfig})
}

// Reconnect proxy
// @Summary Reconnect using the saved proxy
// @Description Rebuilds the WhatsApp connection through the proxy already saved for the instance
// @Tags Instance
// @Accept json
// @Produce json
// @Param instanceId path string true "Instance ID"
// @Success 200 {object} gin.H "Reconnecting through proxy"
// @Failure 400 {object} gin.H "instanceId is required or no proxy configured"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/proxy/{instanceId}/reconnect [post]
func (i *instanceHandler) ReconnectProxy(ctx *gin.Context) {
	instanceId := ctx.Param("instanceId")

	if instanceId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	if err := i.instanceService.ReconnectProxy(instanceId); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

func (i *instanceHandler) DeleteProxy(ctx *gin.Context) {
	instanceId := ctx.Param("instanceId")

	if instanceId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	err := i.instanceService.RemoveProxy(instanceId)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Force reconnect
// @Summary Force reconnect
// @Description Force reconnect
// @Tags Instance
// @Accept json
// @Produce json
// @Param instanceId path string true "Instance Id"
// @Param instance body instance_service.ForceReconnectStruct true "Instance data"
// @Success 200 {object} gin.H "Instance force reconnected successfully"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/forcereconnect/{instanceId} [post]
func (i *instanceHandler) ForceReconnect(ctx *gin.Context) {
	instanceId := ctx.Param("instanceId")

	if instanceId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	var data *instance_service.ForceReconnectStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var number string
	if data.Number == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "number is required"})
		return
	}

	number = data.Number

	err = i.instanceService.ForceReconnect(instanceId, number)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

type GetLogsQuery struct {
	StartDate string `form:"start_date"`
	EndDate   string `form:"end_date"`
	Level     string `form:"level"`
	Limit     int    `form:"limit"`
}

// Get instance logs
// @Summary Read an instance's logs
// @Description Returns the instance's log entries, newest first. All filters are optional:
// @Description omitting them returns the most recent entries. Dates accept RFC3339
// @Description (2026-01-31T00:00:00Z) or a bare YYYY-MM-DD.
// @Tags Instance
// @Produce json
// @Param instanceId path string true "Instance ID"
// @Param start_date query string false "Only entries at or after this moment (RFC3339 or YYYY-MM-DD)"
// @Param end_date query string false "Only entries at or before this moment (RFC3339 or YYYY-MM-DD)"
// @Param level query string false "Filter by severity" Enums(info, warn, error)
// @Param limit query int false "Maximum entries to return" default(100)
// @Success 200 {object} gin.H "Log entries"
// @Failure 400 {object} gin.H "Invalid date or filter"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/logs/{instanceId} [get]
func (h *instanceHandler) GetLogs(c *gin.Context) {
	instanceId := c.Param("instanceId")

	var query GetLogsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Converte as datas
	startDate, err := time.Parse("2006-01-02", query.StartDate)
	if err != nil {
		startDate = time.Now().AddDate(0, 0, -7) // Default: 7 dias atrás
	}

	endDate, err := time.Parse("2006-01-02", query.EndDate)
	if err != nil {
		endDate = time.Now()
	}

	// Ajusta o endDate para o final do dia
	endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 999999999, time.UTC)

	if query.Limit == 0 {
		query.Limit = 100 // Default: 100 registros
	}

	logs, err := h.instanceService.GetLogs(instanceId, startDate, endDate, query.Level, query.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, logs)
}

// GetAdvancedSettings retrieves advanced settings for an instance
// @Summary Get advanced settings
// @Description Get advanced settings for a specific instance
// @Tags Instance
// @Produce json
// @Param instanceId path string true "Instance ID"
// @Success 200 {object} instance_model.AdvancedSettings "Advanced settings retrieved successfully"
// @Failure 400 {object} gin.H "Invalid instance ID"
// @Failure 404 {object} gin.H "Instance not found"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/{instanceId}/advanced-settings [get]
func (h *instanceHandler) GetAdvancedSettings(c *gin.Context) {
	instanceId := c.Param("instanceId")

	if instanceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	settings, err := h.instanceService.GetAdvancedSettings(instanceId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateAdvancedSettings updates advanced settings for an instance
// @Summary Update advanced settings
// @Description Update advanced settings for a specific instance
// @Tags Instance
// @Accept json
// @Produce json
// @Param instanceId path string true "Instance ID"
// @Param settings body instance_model.AdvancedSettings true "Advanced settings data"
// @Success 200 {object} gin.H "Advanced settings updated successfully"
// @Failure 400 {object} gin.H "Invalid request data"
// @Failure 404 {object} gin.H "Instance not found"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/{instanceId}/advanced-settings [put]
func (h *instanceHandler) UpdateAdvancedSettings(c *gin.Context) {
	instanceId := c.Param("instanceId")

	if instanceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	var settings instance_model.AdvancedSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.instanceService.UpdateAdvancedSettings(instanceId, &settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Advanced settings updated successfully",
		"settings": settings,
	})
}

// AddWebhookBody is the request body of the add/remove webhook endpoints.
type AddWebhookBody struct {
	// URL that will receive the instance's events.
	URL string `json:"url" binding:"required" example:"https://meu-servidor.com/webhook"`
}

// Add webhook
// @Summary Add a webhook to an instance
// @Description Registers one more URL to receive this instance's events. An instance may
// @Description have several webhooks and every one receives the same payload. Adding a URL
// @Description that is already registered is a no-op. The change applies immediately, with
// @Description no need to reconnect the instance.
// @Tags Instance
// @Accept json
// @Produce json
// @Param instanceId path string true "Instance ID"
// @Param webhook body instance_handler.AddWebhookBody true "Webhook URL"
// @Success 200 {object} gin.H "Updated webhook list"
// @Failure 400 {object} gin.H "instanceId or url missing"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/webhooks/{instanceId} [post]
func (h *instanceHandler) AddWebhook(c *gin.Context) {
	instanceId := c.Param("instanceId")
	if instanceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}
	var body AddWebhookBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	instance, err := h.instanceService.AddWebhook(instanceId, body.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"webhooks": instance.Webhooks})
}

// Remove webhook
// @Summary Remove a webhook from an instance
// @Description Stops delivering this instance's events to the given URL. Removing a URL that
// @Description is not registered is a no-op. The change applies immediately, with no need to
// @Description reconnect the instance.
// @Tags Instance
// @Accept json
// @Produce json
// @Param instanceId path string true "Instance ID"
// @Param webhook body instance_handler.AddWebhookBody true "Webhook URL to remove"
// @Success 200 {object} gin.H "Updated webhook list"
// @Failure 400 {object} gin.H "instanceId or url missing"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/webhooks/{instanceId} [delete]
func (h *instanceHandler) RemoveWebhook(c *gin.Context) {
	instanceId := c.Param("instanceId")
	if instanceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}
	var body AddWebhookBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	instance, err := h.instanceService.RemoveWebhook(instanceId, body.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"webhooks": instance.Webhooks})
}

// List webhooks
// @Summary List an instance's webhooks
// @Description Returns every URL receiving this instance's events, including the legacy
// @Description single webhook set at creation time alongside any added later.
// @Tags Instance
// @Produce json
// @Param instanceId path string true "Instance ID"
// @Success 200 {object} gin.H "Webhook list"
// @Failure 400 {object} gin.H "instanceId is required"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/webhooks/{instanceId} [get]
func (h *instanceHandler) ListWebhooks(c *gin.Context) {
	instanceId := c.Param("instanceId")
	if instanceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}
	webhooks, err := h.instanceService.ListWebhooks(instanceId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"webhooks": webhooks})
}

func NewInstanceHandler(instanceService instance_service.InstanceService, config *config.Config) InstanceHandler {
	return &instanceHandler{instanceService: instanceService, config: config}
}

// Test proxy
// @Summary Test a proxy
// @Description Checks whether a proxy works and reports the IP it exits from, without
// @Description touching the instance's live connection. Send a proxy in the body to test
// @Description one before saving it; send an empty body to test the instance's saved proxy.
// @Tags Instance
// @Accept json
// @Produce json
// @Param instanceId path string true "Instance ID"
// @Param proxy body instance_service.ProxyConfig false "Proxy to test; omit to use the saved one"
// @Success 200 {object} instance_service.ProxyTestResult "Test result"
// @Failure 400 {object} gin.H "No proxy to test"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /instance/proxy/{instanceId}/test [post]
func (i *instanceHandler) TestProxy(ctx *gin.Context) {
	instanceId := ctx.Param("instanceId")
	if instanceId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	// An empty body means "test what is saved", which is what the Reconnect
	// button next to it acts on. A body means "test what I just typed", so the
	// operator can check a proxy before committing to it.
	var cfg *instance_service.ProxyConfig
	if err := ctx.ShouldBindJSON(&cfg); err != nil || cfg == nil || cfg.Host == "" {
		saved, err := i.instanceService.GetProxy(instanceId)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if saved == nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "no proxy configured for this instance"})
			return
		}
		cfg = saved
	}

	result, err := i.instanceService.TestProxy(cfg)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, result)
}
