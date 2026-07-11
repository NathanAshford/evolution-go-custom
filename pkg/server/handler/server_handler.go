package server_handler

import "github.com/gin-gonic/gin"

type ServerHandler interface {
	ServerOk(ctx *gin.Context)
}

type serverHandler struct {
}

// ServerOk implements ServerHandler.
func (s *serverHandler) ServerOk(ctx *gin.Context) {
	ctx.JSON(200, gin.H{
		"status": "ok",
	})
}

func NewServerHandler() ServerHandler {
	return &serverHandler{}
}
