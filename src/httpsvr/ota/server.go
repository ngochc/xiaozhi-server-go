package ota

import (
	"context"

	"github.com/gin-gonic/gin"
)

type DefaultOTAService struct {
	UpdateURL string
}

// NewDefaultOTAService constructor
func NewDefaultOTAService(updateURL string) *DefaultOTAService {
	return &DefaultOTAService{UpdateURL: updateURL}
}

// Start registers OTA routes
func (s *DefaultOTAService) Start(ctx context.Context, engine *gin.Engine, apiGroup *gin.RouterGroup) error {

	apiGroup.Any("/ota/", s.HandleOTARequest())

	apiGroup.GET("/ota_bin/*filepath", s.HandleFirmwareDownload())

	return nil
}
