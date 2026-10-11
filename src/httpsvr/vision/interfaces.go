package vision

import (
	"context"

	"github.com/gin-gonic/gin"
)

// VisionService defines Vision service interface
type VisionService interface {
	// Start registers Vision routes to engine and apiGroup
	Start(ctx context.Context, engine *gin.Engine, apiGroup *gin.RouterGroup) error
}
