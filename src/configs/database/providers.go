package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/models"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func InitProviders(db *gorm.DB, config *configs.Config) error {
	if err := InitLLMProviders(db, config); err != nil {
		return fmt.Errorf("failed to initialize LLM providers: %v", err)
	}
	if err := InitASRProviders(db, config); err != nil {
		return fmt.Errorf("failed to initialize ASR providers: %v", err)
	}
	if err := InitTTSProviders(db, config); err != nil {
		return fmt.Errorf("failed to initialize TTS providers: %v", err)
	}
	if err := InitVLLLMProviders(db, config); err != nil {
		return fmt.Errorf("failed to initialize VLLM providers: %v", err)
	}
	return nil
}

func InitTTSProviders(db *gorm.DB, config *configs.Config) error {
	if config == nil || config.TTS == nil {
		return nil
	}

	for name, provider := range config.TTS {
		providerJson, err := json.Marshal(provider)
		if err != nil {
			return fmt.Errorf("failed to serialize TTS provider %s config: %v", name, err)
		}
		ttsConfig := models.TTSConfig{
			Name: name,
			Type: provider.Type,
			Data: datatypes.JSON(providerJson),
		}

		if err := db.Where("name = ?", name).FirstOrCreate(&ttsConfig).Error; err != nil {
			return fmt.Errorf("failed to insert/update TTS provider %s: %v", name, err)
		}
	}

	return nil
}

func InitLLMProviders(db *gorm.DB, config *configs.Config) error {
	if config == nil || config.LLM == nil {
		return nil
	}
	for name, provider := range config.LLM {
		providerJson, err := json.Marshal(provider)
		if err != nil {
			return fmt.Errorf("failed to serialize LLM provider %s config: %v", name, err)
		}
		llmConfig := models.LLMConfig{
			Name: name,
			Type: provider.Type,
			Data: datatypes.JSON(providerJson),
		}
		// Compare and create or update by name
		if err := db.Where("name = ?", name).FirstOrCreate(&llmConfig).Error; err != nil {
			return fmt.Errorf("failed to insert/update LLM provider %s: %v", name, err)
		}
	}

	return nil
}

func InitASRProviders(db *gorm.DB, config *configs.Config) error {
	if config == nil || config.ASR == nil {
		return nil
	}

	for name, provider := range config.ASR {
		providerJson, err := json.Marshal(provider)
		if err != nil {
			return fmt.Errorf("failed to serialize ASR provider %s config: %v", name, err)
		}
		providerType := ""
		if typeVal, ok := provider["type"]; ok {
			if typeStr, ok := typeVal.(string); ok {
				providerType = typeStr
			}
		}

		asrConfig := models.ASRConfig{
			Name: name,
			Type: providerType,
			Data: datatypes.JSON(providerJson),
		}

		if err := db.Where("name = ?", name).FirstOrCreate(&asrConfig).Error; err != nil {
			return fmt.Errorf("failed to insert/update ASR provider %s: %v", name, err)
		}
	}

	return nil
}

func InitVLLLMProviders(db *gorm.DB, config *configs.Config) error {
	if config == nil || config.VLLLM == nil {
		return nil
	}
	for name, provider := range config.VLLLM {
		providerJson, err := json.Marshal(provider)
		if err != nil {
			return fmt.Errorf("failed to serialize VLLM provider %s config: %v", name, err)
		}

		vlllmConfig := models.VLLLMConfig{
			Name: name,
			Type: provider.Type, // Reuses LLM type
			Data: datatypes.JSON(providerJson),
		}

		if err := db.Where("name = ?", name).FirstOrCreate(&vlllmConfig).Error; err != nil {
			return fmt.Errorf("failed to insert/update VLLM provider %s: %v", name, err)
		}
	}

	return nil
}

func GetAllProviders(userID uint) map[string]string {
	providersMap := make(map[string]string)
	asr, _ := GetProviderByType("ASR", userID)
	tts, _ := GetProviderByType("TTS", userID)
	llm, _ := GetProviderByType("LLM", userID)
	vlllm, _ := GetProviderByType("VLLLM", userID)
	for name, data := range asr {
		providersMap["ASR:"+name] = data
	}
	for name, data := range tts {
		providersMap["TTS:"+name] = data
	}
	for name, data := range llm {
		providersMap["LLM:"+name] = data
	}
	for name, data := range vlllm {
		providersMap["VLLLM:"+name] = data
	}

	return providersMap
}

func RemoveSensitiveFields(data datatypes.JSON) (string, error) {
	configData := make(map[string]interface{})
	if err := json.Unmarshal(data, &configData); err != nil {
		return "", fmt.Errorf("failed to deserialize ASR provider data: %v", err)
	}
	// Remove sensitive fields
	delete(configData, "token")
	delete(configData, "access_token")
	delete(configData, "api_key")
	delete(configData, "appid")

	// Reserialize sanitized data to JSON string
	configJson, err := json.Marshal(configData)
	if err != nil {
		return "", fmt.Errorf("failed to serialize sanitized ASR provider data: %v", err)
	}
	return string(configJson), nil
}

func GetProviderNameList(userID uint) (asrList, ttsList, llmList, vlllmList []string) {
	asrList = make([]string, 0)
	var asrConfigs []models.ASRConfig
	if err := DB.Find(&asrConfigs).Error; err == nil {
		for _, asr := range asrConfigs {
			if asr.UserID != 0 && asr.UserID != AdminUserID && asr.UserID != userID {
				continue
			}
			asrList = append(asrList, asr.Name)
		}
	}

	ttsList = make([]string, 0)
	var ttsConfigs []models.TTSConfig
	if err := DB.Find(&ttsConfigs).Error; err == nil {
		for _, tts := range ttsConfigs {
			if tts.UserID != 0 && tts.UserID != AdminUserID && tts.UserID != userID {
				continue
			}
			ttsList = append(ttsList, tts.Name)
		}
	}

	llmList = make([]string, 0)
	var llmConfigs []models.LLMConfig
	if err := DB.Find(&llmConfigs).Error; err == nil {
		for _, llm := range llmConfigs {
			if llm.UserID != 0 && llm.UserID != AdminUserID && llm.UserID != userID {
				continue
			}
			llmList = append(llmList, llm.Name)
		}
	}

	vlllmList = make([]string, 0)
	var vlllmConfigs []models.VLLLMConfig
	if err := DB.Find(&vlllmConfigs).Error; err == nil {
		for _, vlllm := range vlllmConfigs {
			if vlllm.UserID != 0 && vlllm.UserID != AdminUserID && vlllm.UserID != userID {
				continue
			}
			vlllmList = append(vlllmList, vlllm.Name)
		}
	}
	return asrList, ttsList, llmList, vlllmList
}

func GetProviderByType(providerType string, userID uint) (map[string]string, error) {
	return GetProviderByTypeInternal(providerType, userID, true)
}

func GetProviderByTypeInternal(providerType string, userID uint, bRemoveSensitive bool) (map[string]string, error) {
	// Return providers list by provider type
	switch providerType {
	case "ASR":
		// Query ASR provider names from database
		var asrConfigs []models.ASRConfig
		if err := DB.Find(&asrConfigs).Error; err != nil {
			return nil, fmt.Errorf("failed to query ASR providers: %v", err)
		}
		asrs := make(map[string]string)
		for _, config := range asrConfigs {
			if config.UserID != 0 && config.UserID != AdminUserID && config.UserID != userID {
				continue
			}
			if bRemoveSensitive {
				sanitized, _ := RemoveSensitiveFields(config.Data)
				asrs[config.Name] = normalizeProviderDataType(config.Name, config.Type, datatypes.JSON([]byte(sanitized)))
			} else {
				asrs[config.Name] = normalizeProviderDataType(config.Name, config.Type, config.Data)
			}
		}
		return asrs, nil
	case "TTS":
		// Query TTS provider names from database
		var ttsConfigs []models.TTSConfig
		if err := DB.Find(&ttsConfigs).Error; err != nil {
			return nil, fmt.Errorf("failed to query TTS providers: %v", err)
		}
		tts := make(map[string]string)
		for _, config := range ttsConfigs {
			if config.UserID != 0 && config.UserID != AdminUserID && config.UserID != userID {
				continue
			}
			if bRemoveSensitive {
				tts[config.Name], _ = RemoveSensitiveFields(config.Data) // string(config.Data)
			} else {
				tts[config.Name] = string(config.Data)
			}
		}
		return tts, nil
	case "LLM":
		// Query LLM provider names from database
		var llmConfigs []models.LLMConfig
		if err := DB.Find(&llmConfigs).Error; err != nil {
			return nil, fmt.Errorf("failed to query LLM providers: %v", err)
		}
		llms := make(map[string]string)
		for _, config := range llmConfigs {
			if config.UserID != 0 && config.UserID != AdminUserID && config.UserID != userID {
				continue
			}
			if bRemoveSensitive {
				sanitized, _ := RemoveSensitiveFields(config.Data)
				llms[config.Name] = normalizeProviderNumericData("LLM", datatypes.JSON([]byte(sanitized)))
			} else {
				llms[config.Name] = normalizeProviderNumericData("LLM", config.Data)
			}
		}
		return llms, nil
	case "VLLLM":
		// Query VLLM provider names from database
		var vlllmConfigs []models.VLLLMConfig
		if err := DB.Find(&vlllmConfigs).Error; err != nil {
			return nil, fmt.Errorf("failed to query VLLM providers: %v", err)
		}
		vlllms := make(map[string]string)
		for _, config := range vlllmConfigs {
			if config.UserID != 0 && config.UserID != AdminUserID && config.UserID != userID {
				continue
			}
			if bRemoveSensitive {
				sanitized, _ := RemoveSensitiveFields(config.Data)
				vlllms[config.Name] = normalizeProviderNumericData("VLLLM", datatypes.JSON([]byte(sanitized)))
			} else {
				vlllms[config.Name] = normalizeProviderNumericData("VLLLM", config.Data)
			}
		}
		return vlllms, nil
	default:
		return nil, fmt.Errorf("unknown provider type: %s", providerType)
	}
}

// CreateProvider creates a new provider
func normalizeProviderDataType(name, providerType string, data datatypes.JSON) string {
	if providerType == "" {
		return string(data)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return string(data)
	}

	dataType, _ := config["type"].(string)
	if dataType != "" && dataType != providerType && utils.DefaultLogger != nil {
		utils.DefaultLogger.Warn(
			"[Provider] Config type mismatch, used database type field: name=%s db_type=%s data_type=%s",
			name,
			providerType,
			dataType,
		)
	}
	config["type"] = providerType

	normalized, err := json.Marshal(config)
	if err != nil {
		return string(data)
	}
	return string(normalized)
}

// normalizeProviderNumericData normalizes numeric fields stored as strings in legacy configs.
// Ensures normalization on read and clean numeric JSON on subsequent writes.
func normalizeProviderNumericData(providerType string, data datatypes.JSON) string {
	if providerType != "LLM" && providerType != "VLLLM" {
		return string(data)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return string(data)
	}
	normalizeProviderNumericFields(config)
	normalized, err := json.Marshal(config)
	if err != nil {
		return string(data)
	}
	return string(normalized)
}

func normalizeProviderNumericFields(config map[string]interface{}) {
	if value, ok := config["max_tokens"].(string); ok {
		if number, err := strconv.Atoi(value); err == nil {
			config["max_tokens"] = number
		}
	}
	for _, field := range []string{"temperature", "top_p"} {
		if value, ok := config[field].(string); ok {
			if number, err := strconv.ParseFloat(value, 64); err == nil {
				config[field] = number
			}
		}
	}
}

func normalizeProviderInput(providerType string, data interface{}) interface{} {
	if providerType != "LLM" && providerType != "VLLLM" {
		return data
	}
	config, ok := data.(map[string]interface{})
	if !ok {
		return data
	}
	copyConfig := make(map[string]interface{}, len(config))
	for key, value := range config {
		copyConfig[key] = value
	}
	normalizeProviderNumericFields(copyConfig)
	return copyConfig
}

func CreateProvider(providerType, name string, data interface{}, createUserID uint) error {
	// Personal edition: all providers belong to single local admin user.
	createUserID = AdminUserID
	data = normalizeProviderInput(providerType, data)
	providerJson, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to serialize provider data: %v", err)
	}

	// Extract type field from providerJson
	var providerSubType string
	if typeVal, ok := data.(map[string]interface{})["type"]; ok {
		if typeStr, ok := typeVal.(string); ok {
			providerSubType = typeStr
		}
	} else {
		return fmt.Errorf("missing type field in provider data")
	}

	switch providerType {
	case "ASR":
		config := models.ASRConfig{
			UserID: createUserID,
			Name:   name,
			Type:   providerSubType,
			Data:   datatypes.JSON(providerJson),
		}
		return DB.Create(&config).Error
	case "TTS":
		config := models.TTSConfig{
			UserID: createUserID,
			Name:   name,
			Type:   providerSubType,
			Data:   datatypes.JSON(providerJson),
		}
		return DB.Create(&config).Error
	case "LLM":
		config := models.LLMConfig{
			UserID: createUserID,
			Name:   name,
			Type:   providerSubType,
			Data:   datatypes.JSON(providerJson),
		}
		return DB.Create(&config).Error
	case "VLLLM":
		config := models.VLLLMConfig{
			UserID: createUserID,
			Name:   name,
			Type:   providerSubType,
			Data:   datatypes.JSON(providerJson),
		}
		return DB.Create(&config).Error
	default:
		return fmt.Errorf("unknown provider type: %s", providerType)
	}
}

// UpdateProvider updates a provider
func UpdateProvider(providerType, name string, data interface{}, userID uint) error {
	data = normalizeProviderInput(providerType, data)
	providerJson, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to serialize provider data: %v", err)
	}

	switch providerType {
	case "ASR":
		result := DB.Model(&models.ASRConfig{}).
			Where(&models.ASRConfig{Name: name, UserID: userID}).
			Update("data", datatypes.JSON(providerJson))

		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("ASR provider %s does not exist or permission denied", name)
		}
		return nil
	case "TTS":
		result := DB.Model(&models.TTSConfig{}).
			Where(&models.TTSConfig{Name: name, UserID: userID}).
			Update("data", datatypes.JSON(providerJson))

		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("TTS provider %s does not exist or permission denied", name)
		}
		return nil
	case "LLM":
		result := DB.Model(&models.LLMConfig{}).
			Where(&models.LLMConfig{Name: name, UserID: userID}).
			Update("data", datatypes.JSON(providerJson))

		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("LLM provider %s does not exist or permission denied", name)
		}
		return nil
	case "VLLLM":
		result := DB.Model(&models.VLLLMConfig{}).
			Where(&models.VLLLMConfig{Name: name, UserID: userID}).
			Update("data", datatypes.JSON(providerJson))

		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("VLLM provider %s does not exist or permission denied", name)
		}
		return nil
	default:
		return fmt.Errorf("unknown provider type: %s", providerType)
	}
}

// DeleteProvider deletes a provider
var ErrNoPermission = errors.New("permission denied")

func DeleteProvider(providerType, name string, userID uint) error {
	switch providerType {
	case "ASR":
		var cfg models.ASRConfig
		if err := DB.Where("name = ?", name).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return gorm.ErrRecordNotFound
			}
			return err
		}
		if cfg.UserID != userID && userID != AdminUserID {
			return ErrNoPermission
		}
		return DB.Delete(&cfg).Error
	case "TTS":
		var cfg models.TTSConfig
		if err := DB.Where("name = ?", name).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return gorm.ErrRecordNotFound
			}
			return err
		}
		if cfg.UserID != userID && userID != AdminUserID {
			return ErrNoPermission
		}
		return DB.Delete(&cfg).Error
	case "LLM":
		var cfg models.LLMConfig
		if err := DB.Where("name = ?", name).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return gorm.ErrRecordNotFound
			}
			return err
		}
		if cfg.UserID != userID && userID != AdminUserID {
			return ErrNoPermission
		}
		return DB.Delete(&cfg).Error
	case "VLLLM":
		var cfg models.VLLLMConfig
		if err := DB.Where("name = ?", name).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return gorm.ErrRecordNotFound
			}
			return err
		}
		if cfg.UserID != userID && userID != AdminUserID {
			return ErrNoPermission
		}
		return DB.Delete(&cfg).Error
	default:
		return fmt.Errorf("unknown provider type: %s", providerType)
	}
}

// GetProviderByName retrieves a specific provider by name
func GetProviderByName(providerType, name string) (string, error) {
	switch providerType {
	case "ASR":
		var config models.ASRConfig
		if err := DB.Where(&models.ASRConfig{Name: name}).First(&config).Error; err != nil {
			return "", err
		}
		return RemoveSensitiveFields(config.Data)
	case "TTS":
		var config models.TTSConfig
		if err := DB.Where(&models.TTSConfig{Name: name}).First(&config).Error; err != nil {
			return "", err
		}
		return RemoveSensitiveFields(config.Data)
	case "LLM":
		var config models.LLMConfig
		if err := DB.Where(&models.LLMConfig{Name: name}).First(&config).Error; err != nil {
			return "", err
		}
		return RemoveSensitiveFields(config.Data)
	case "VLLLM":
		var config models.VLLLMConfig
		if err := DB.Where(&models.VLLLMConfig{Name: name}).First(&config).Error; err != nil {
			return "", err
		}
		return RemoveSensitiveFields(config.Data)
	default:
		return "", fmt.Errorf("unknown provider type: %s", providerType)
	}
}
