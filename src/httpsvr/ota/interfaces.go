package ota

import (
	"context"

	"github.com/gin-gonic/gin"
)

// OTAService defines the OTA service interface
type OTAService interface {
	// Start registers OTA routes to engine and apiGroup
	Start(ctx context.Context, engine *gin.Engine, apiGroup *gin.RouterGroup) error
}
