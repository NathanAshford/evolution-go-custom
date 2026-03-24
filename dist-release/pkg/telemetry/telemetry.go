package telemetry

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type TelemetryData struct {
	Route      string    `json:"route"`
	APIVersion string    `json:"apiVersion"`
	Timestamp  time.Time `json:"timestamp"`
}

type telemetryService struct{}

func (t *telemetryService) TelemetryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		route := c.FullPath()
		go SendTelemetry(route)
		c.Next()
	}
}

type TelemetryService interface {
	TelemetryMiddleware() gin.HandlerFunc
}

func SendTelemetry(route string) {
	if route == "/" {
		return
	}

	telemetry := TelemetryData{
		Route:      route,
		APIVersion: "evo-go",
		Timestamp:  time.Now(),
	}

	url := "https://log.evolution-api.com/telemetry"

	data, err := json.Marshal(telemetry)
	if err != nil {
		log.Println("Erro ao serializar telemetria:", err)
		return
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Println("Erro ao enviar telemetria:", err)
		return
	}
	defer resp.Body.Close()
}

func NewTelemetryService() TelemetryService {
	return &telemetryService{}
}
