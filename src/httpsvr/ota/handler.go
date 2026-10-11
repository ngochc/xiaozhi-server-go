package ota

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/httpsvr/webapi"
	"xiaozhi-server-go/src/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// OtaFirmwareResponse defines OTA firmware endpoint response structure
type OtaFirmwareResponse struct {
	ServerTime struct {
		Timestamp      int64 `json:"timestamp" example:"1688443200000"`
		TimezoneOffset int   `json:"timezone_offset" example:"480"`
	} `json:"server_time"`
	Firmware struct {
		Version string `json:"version" example:"1.0.3"`
		URL     string `json:"url" example:"/ota_bin/1.0.3.bin"`
	} `json:"firmware"`
	Websocket struct {
		URL string `json:"url" example:"wss://example.com/ota"`
	} `json:"websocket"`
}

// ErrorResponse defines error response structure
type ErrorResponse struct {
	Success bool   `json:"success" example:"false"`
	Message string `json:"message" example:"Missing device-id"`
}

// HandleOTARequest handles OTA requests (POST /ota/)
//
// @Summary Device OTA request
// @Description Device initiates OTA upgrade request; server returns latest firmware info and connection endpoints.
// @Tags OTA
// @Accept json
// @Produce json
// @Param device-id header string true "Unique device ID such as MAC address"
// @Param client-id header string false "Client ID identifying application or user"
// @Param body body ota.OTARequestBody false "Application version information"
// @Success 200 {object} ota.OTAResponse
// @Failure 400 {object} object "Missing required parameter or invalid body"
// @Router /ota/ [post]
func (s *DefaultOTAService) HandleOTARequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodOptions:
			c.Status(http.StatusOK)
			return
		case http.MethodGet:
			fmt.Println("OTA interface accessed, websocket address:", s.UpdateURL)
			c.String(http.StatusOK, "OTA interface is running, websocket address: "+s.UpdateURL)
			return
		case http.MethodPost:
			s.handlePostOTA(c)
			return
		default:
			c.String(http.StatusMethodNotAllowed, "Method not allowed: %s", c.Request.Method)
		}
	}
}

// Trans2OTARequestBody converts raw request body to OTARequestBody struct
// Handles compatibility across different payload formats
func (s *DefaultOTAService) Trans2OTARequestBody(raw map[string]interface{}) OTARequestBody {
	var req OTARequestBody
	// Attempt direct unmarshal first
	if b, err := json.Marshal(raw); err == nil {
		if err := json.Unmarshal(b, &req); err == nil {
			return req
		} else {
			utils.DefaultLogger.Warn("Failed to convert OTARequestBody: %v", err)
		}
	} else {
		utils.DefaultLogger.Error("Failed to JSON-encode OTARequestBody: %v", err)
	}

	utils.DefaultLogger.Warn("Direct conversion to OTARequestBody failed, attempting field fallback")
	// Fallback to field-by-field conversion
	// Application
	if app, ok := raw["application"].(map[string]interface{}); ok {
		if v, ok := app["name"].(string); ok {
			req.Application.Name = v
		}
		if v, ok := app["version"].(string); ok {
			req.Application.Version = v
		}
		if v, ok := app["compile_time"].(string); ok {
			req.Application.CompileTime = v
		}
		if v, ok := app["elf_sha256"].(string); ok {
			req.Application.ElfSHA256 = v
		}
		if v, ok := app["idf_version"].(string); ok {
			req.Application.IDFVersion = v
		}
	}
	// Board
	if board, ok := raw["board"].(map[string]interface{}); ok {
		if v, ok := board["channel"].(float64); ok {
			req.Board.Channel = int(v)
		}
		if v, ok := board["ip"].(string); ok {
			req.Board.IP = v
		}
		if v, ok := board["mac"].(string); ok {
			req.Board.MAC = v
		}
		if v, ok := board["name"].(string); ok {
			req.Board.Name = v
		}
		if v, ok := board["rssi"].(float64); ok {
			req.Board.RSSI = int(v)
		}
		if v, ok := board["ssid"].(string); ok {
			req.Board.SSID = v
		}
		if v, ok := board["type"].(string); ok {
			req.Board.Type = v
		}
	}
	// ChipInfo
	if chip, ok := raw["chip_info"].(map[string]interface{}); ok {
		if v, ok := chip["cores"].(float64); ok {
			req.ChipInfo.Cores = int(v)
		}
		if v, ok := chip["features"].(float64); ok {
			req.ChipInfo.Features = int(v)
		}
		if v, ok := chip["model"].(float64); ok {
			req.ChipInfo.Model = int(v)
		}
		if v, ok := chip["revision"].(float64); ok {
			req.ChipInfo.Revision = int(v)
		}
	}
	if v, ok := raw["chip_model_name"].(string); ok {
		req.ChipModelName = v
	}
	if v, ok := raw["flash_size"].(float64); ok {
		req.FlashSize = v
	}
	if v, ok := raw["language"].(string); ok {
		req.Language = v
	}
	if v, ok := raw["mac_address"].(string); ok {
		req.MacAddress = v
	}
	if v, ok := raw["minimum_free_heap_size"]; ok {
		switch vv := v.(type) {
		case string:
			req.MinimumFreeHeapSize = StringOrNumber(vv)
		case float64:
			req.MinimumFreeHeapSize = StringOrNumber(fmt.Sprintf("%v", vv))
		}
	}
	if ota, ok := raw["ota"].(map[string]interface{}); ok {
		if v, ok := ota["label"].(string); ok {
			req.OTA.Label = v
		}
	}
	if pt, ok := raw["partition_table"].([]interface{}); ok {
		for _, item := range pt {
			if m, ok := item.(map[string]interface{}); ok {
				var p struct {
					Address float64 `json:"address"`
					Label   string  `json:"label"`
					Size    float64 `json:"size"`
					Subtype int     `json:"subtype"`
					Type    int     `json:"type"`
				}
				if v, ok := m["address"].(float64); ok {
					p.Address = v
				}
				if v, ok := m["label"].(string); ok {
					p.Label = v
				}
				if v, ok := m["size"].(float64); ok {
					p.Size = v
				}
				if v, ok := m["subtype"].(float64); ok {
					p.Subtype = int(v)
				}
				if v, ok := m["type"].(float64); ok {
					p.Type = int(v)
				}
				req.PartitionTable = append(req.PartitionTable, p)
			}
		}
	}
	if v, ok := raw["uuid"].(string); ok {
		req.UUID = v
	}
	if v, ok := raw["version"].(float64); ok {
		req.Version = int(v)
	}
	return req
}

// @Summary Upload device info to get latest firmware
// @Description Returns latest firmware version and download URL after uploading device info
// @Tags OTA
// @Accept json
// @Produce json
// @Param device-id header string true "Device ID"
// @Param body body OtaRequest true "Request body"
// @Success 200 {object} OtaFirmwareResponse
// @Failure 400 {object} ErrorResponse
// @Router /ota/ [post]
func (s *DefaultOTAService) handlePostOTA(c *gin.Context) {
	deviceID := c.GetHeader("device-id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Success: false, Message: "Missing device-id"})
		return
	}

	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Parse failed: " + err.Error()})
		utils.DefaultLogger.Error("Failed to parse OTA request body: %v", err)
		return
	}
	// Compatibility conversion to OTARequestBody
	var req OTARequestBody = s.Trans2OTARequestBody(raw)

	clientID := c.GetHeader("client-id")
	client_id := "CGID_test@@@" + strings.Replace(deviceID, ":", "_", -1) + "@@@" + clientID
	version := req.Application.Version
	if version == "" {
		version = "1.0.0"
	}

	otaDir := filepath.Join(".", "ota_bin")
	_ = os.MkdirAll(otaDir, 0755)
	bins, _ := filepath.Glob(filepath.Join(otaDir, "*.bin"))
	firmwareURL := ""
	if len(bins) > 0 {
		sort.Slice(bins, func(i, j int) bool {
			return versionLess(bins[j], bins[i])
		})
		latest := filepath.Base(bins[0])
		version = strings.TrimSuffix(latest, ".bin")
		firmwareURL = "/ota_bin/" + latest
	}
	cfg := configs.Cfg
	updateURL := cfg.Web.Websocket
	deviceName := req.Board.Name
	s.CheckAndUpdateDevice(c, cfg, req, deviceID, client_id, deviceName, version)
	resp := OtaFirmwareResponse{}
	resp.ServerTime.Timestamp = time.Now().UnixNano() / 1e6
	resp.ServerTime.TimezoneOffset = 8 * 60
	resp.Firmware.Version = version
	resp.Firmware.URL = firmwareURL
	resp.Websocket.URL = updateURL
	if resp.Websocket.URL == "" {
		utils.DefaultLogger.Warn("===========================================================")
		utils.DefaultLogger.Warn("=====  WebSocket URL not configured, OTA service may not work properly =====")
		utils.DefaultLogger.Warn("=====  Please configure WebSocket URL and restart service            =====")
		utils.DefaultLogger.Warn("===========================================================")
	}

	c.JSON(http.StatusOK, resp)
}
func (s *DefaultOTAService) CheckAndUpdateDevice(
	c *gin.Context,
	cfg *configs.Config,
	req OTARequestBody,
	deviceID, clientID, deviceName, version string,
) *models.Device {
	var resultDevice *models.Device
	macAddress := database.NormalizeMACAddress(req.MacAddress)
	if macAddress == "" {
		macAddress = database.NormalizeMACAddress(req.Board.MAC)
	}
	if macAddress == "" {
		macAddress = database.NormalizeMACAddress(deviceID)
	}
	webapi.WithTx(c, func(tx *gorm.DB) error {
		// Check if device is already registered
		device, err := database.FindDeviceByID(tx, deviceID) // Ensure device exists
		if err != nil {
			if device == nil {
				// Check if device was soft-deleted
				deleteDevice, err := database.FindDeletedDeviceByID(tx, deviceID)
				if err != nil && err != gorm.ErrRecordNotFound {
					utils.DefaultLogger.Error("Failed to query deleted device: %v", err)
				}
				if deleteDevice != nil {
					// Hard delete
					if err := database.HardDeleteDevice(tx, deleteDevice.DeviceID); err != nil {
						utils.DefaultLogger.Error("Failed to hard delete device %s: %v", deviceID, err)
						c.JSON(
							http.StatusInternalServerError,
							gin.H{"success": false, "message": "Device status error, contact administrator: " + err.Error()},
						)
						return err
					}
				}
			}

			device = &models.Device{
				DeviceID:         deviceID,   // Set device ID
				MACAddress:       macAddress, // Physical MAC address reported by device
				ClientID:         clientID,   // Set client ID
				Name:             deviceName, // Set device name
				Version:          version,    // Set device version
				RegisterTimeV2:   time.Now(),
				LastActiveTimeV2: time.Now(),
				BoardType:        req.Board.Type,    // Set board type
				ChipModelName:    req.ChipModelName, // Set chip model
				Channel:          req.Board.Channel, // Set WiFi channel
				SSID:             req.Board.SSID,    // Set WiFi SSID
				Language:         req.Language,      // Set language
				OTA:              true,              // Enable OTA upgrade
			}
			ownerID := database.AdminUserID
			device.UserID = &ownerID
			// New devices bind to Setup Assistant once created by admin; devices do not create it implicitly.
			if onboardingAgent, err := database.GetOnboardingAgent(tx); err == nil {
				ownerID := onboardingAgent.UserID
				device.AgentID = &onboardingAgent.ID
				device.UserID = &ownerID
				device.AuthStatus = "onboarding"
			} else if err != gorm.ErrRecordNotFound {
				utils.DefaultLogger.Error("Failed to query Setup Assistant: %v", err)
				return err
			}
			appBytes, _ := json.Marshal(req.Application)
			device.Application = string(appBytes)
			if err := database.AddDevice(tx, device); err != nil { // Save device info
				utils.DefaultLogger.Error("Failed to save device information: %v", err)
				c.JSON(
					http.StatusInternalServerError,
					gin.H{"success": false, "message": "Failed to save device information: " + err.Error()},
				)
				return err
			} else {
				utils.DefaultLogger.Info("New device registered successfully: %s", deviceID)
			}
		}

		appBytes, _ := json.Marshal(req.Application)
		if device.Application != string(appBytes) || (macAddress != "" && device.MACAddress != macAddress) {
			device.Application = string(appBytes) // Update application info
			if macAddress != "" {
				device.MACAddress = macAddress
			}
			if err := database.UpdateDevice(tx, device); err != nil {
				utils.DefaultLogger.Error("Failed to update device application info: %v", err)
			}
		}
		resultDevice = device
		return nil
	})
	return resultDevice
}

// HandleFirmwareDownload handles /ota_bin/:filename downloads
func (s *DefaultOTAService) HandleFirmwareDownload() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Supports both /ota_bin/:filename and wildcard /ota_bin/*filepath
		// Read wildcard parameter first, fallback to legacy param
		reqPath := c.Param("filepath")

		if reqPath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid file path"})
			return
		}

		clean := path.Clean(reqPath)
		clean = strings.TrimPrefix(clean, "/")
		if strings.Contains(clean, "..") {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid file path"})
			return
		}

		p := filepath.Join("ota_bin", filepath.FromSlash(clean))
		//fmt.Println("Firmware download requested:", clean, "full path:", p)

		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "file not found"})
			return
		}

		c.Header("Content-Type", "application/octet-stream")
		// Use Gin File helper to handle range requests and streaming efficiently
		c.File(p)
	}
}

// Compares two semantic version strings (a < b)
func versionLess(a, b string) bool {
	av := strings.Split(strings.TrimSuffix(filepath.Base(a), ".bin"), ".")
	bv := strings.Split(strings.TrimSuffix(filepath.Base(b), ".bin"), ".")

	maxLen := len(av)
	if len(bv) > maxLen {
		maxLen = len(bv)
	}

	for i := 0; i < maxLen; i++ {
		var ai, bi int
		if i < len(av) {
			ai, _ = strconv.Atoi(av[i])
		}
		if i < len(bv) {
			bi, _ = strconv.Atoi(bv[i])
		}
		if ai != bi {
			return ai < bi
		}
	}
	return false
}
