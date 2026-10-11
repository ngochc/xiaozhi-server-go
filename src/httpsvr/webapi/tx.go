package webapi

// Generic transaction wrapper function
import (
	"net/http"
	"xiaozhi-server-go/src/configs/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// WithTx executes operation within database transaction
// c: gin.Context, fn: func(tx *gorm.DB) error
func WithTx(c *gin.Context, fn func(tx *gorm.DB) error) {
	tx := database.GetTxDB()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "transaction aborted unexpectedly"})
		}
	}()
	if err := fn(tx); err != nil {
		tx.Rollback()
		return
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction commit failed: " + err.Error()})
		return
	}
}

// Transaction wrapper without gin context
func WithTxNoContext(fn func(tx *gorm.DB) error) error {
	tx := database.GetTxDB()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}
