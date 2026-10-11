package configs

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the primary server configuration.
type Config struct {
	Server struct {
		IP    string `yaml:"ip" json:"ip"`
		Port  int    `yaml:"port" json:"port"`
		Token string `json:"token"`
		Auth  struct {
			Enabled bool `yaml:"enabled" json:"enabled"`
			Store   struct {
				Type   string `yaml:"type" json:"type"`     // memory/file/redis
				Expiry int    `yaml:"expiry" json:"expiry"` // Expiry in hours
			} `yaml:"store" json:"store"`
			AllowedDevices []string `yaml:"allowed_devices" json:"allowed_devices"`
			Tokens         []string `yaml:"tokens" json:"tokens"`
		} `yaml:"auth" json:"auth"`
		ServerVersion string `yaml:"server_version" json:"server_version"`
	} `yaml:"server" json:"server"`

	// Transport configuration
	Transport struct {
		WebSocket struct {
			Enabled bool   `yaml:"enabled" json:"enabled"`
			IP      string `yaml:"ip" json:"ip"`
			Port    int    `yaml:"port" json:"port"`
		} `yaml:"websocket" json:"websocket"`

		MQTTUDP struct {
			Enabled bool `yaml:"enabled" json:"enabled"`
			MQTT    struct {
				IP   string `yaml:"ip" json:"ip"`
				Port int    `yaml:"port" json:"port"`
				QoS  int    `yaml:"qos" json:"qos"`
			} `yaml:"mqtt" json:"mqtt"`
			UDP struct {
				IP                string `yaml:"ip" json:"ip"`
				ShowPort          int    `yaml:"show_port" json:"show_port"` // Display port
				Port              int    `yaml:"port" json:"port"`
				SessionTimeout    string `yaml:"session_timeout" json:"session_timeout"`
				MaxPacketSize     int    `yaml:"max_packet_size" json:"max_packet_size"`
				EnableReliability bool   `yaml:"enable_reliability" json:"enable_reliability"`
			} `yaml:"udp" json:"udp"`
		} `yaml:"mqtt_udp" json:"mqtt_udp"`
	} `yaml:"transport" json:"transport"`

	Log struct {
		LogFormat string `yaml:"log_format" json:"log_format"`
		LogLevel  string `yaml:"log_level" json:"log_level"`
		LogDir    string `yaml:"log_dir" json:"log_dir"`
		LogFile   string `yaml:"log_file" json:"log_file"`
	} `yaml:"log" json:"log"`

	Web struct {
		Enabled      bool   `yaml:"enabled" json:"enabled"`
		Port         int    `yaml:"port" json:"port"`
		StaticDir    string `yaml:"static_dir" json:"static_dir"`
		Websocket    string `yaml:"websocket" json:"websocket"`
		VisionURL    string `yaml:"vision" json:"vision"`
		ActivateText string `yaml:"activate_text" json:"activate_text"` // Text delivered with activation code
	} `yaml:"web" json:"web"`

	DefaultPrompt   string        `yaml:"prompt"             json:"prompt"`
	Roles           []Role        `yaml:"roles"              json:"roles"` // Roles list
	DeleteAudio     bool          `yaml:"delete_audio"       json:"delete_audio"`
	QuickReply      bool          `yaml:"quick_reply"        json:"quick_reply"`
	QuickReplyWords []string      `yaml:"quick_reply_words"  json:"quick_reply_words"`
	LocalMCPFun     []LocalMCPFun `yaml:"local_mcp_fun"      json:"local_mcp_fun"` // Local MCP functions
	SaveTTSAudio    bool          `yaml:"save_tts_audio"  json:"save_tts_audio"`   // Save outbound TTS audio
	SaveUserAudio   bool          `yaml:"save_user_audio" json:"save_user_audio"`  // Save inbound user audio

	SelectedModule map[string]string `yaml:"selected_module" json:"selected_module"`

	ASR   map[string]ASRConfig  `yaml:"ASR"   json:"ASR"`
	TTS   map[string]TTSConfig  `yaml:"TTS"   json:"TTS"`
	LLM   map[string]LLMConfig  `yaml:"LLM"   json:"LLM"`
	VLLLM map[string]VLLMConfig `yaml:"VLLLM" json:"VLLLM"`

	CMDExit []string `yaml:"CMD_exit" json:"CMD_exit"`
}

// ErrInitialConfigCreated indicates that the initial private config was generated on first launch.
var ErrInitialConfigCreated = errors.New("initial configuration file generated on first start")

type LocalMCPFun struct {
	Name        string `yaml:"name"         json:"name"`        // Function name
	Description string `yaml:"description"  json:"description"` // Function description
	Enabled     bool   `yaml:"enabled"      json:"enabled"`     // Enabled status
}

type Role struct {
	Name        string `yaml:"name"         json:"name"`        // Role name
	Description string `yaml:"description"  json:"description"` // Role description
	Enabled     bool   `yaml:"enabled"      json:"enabled"`     // Enabled status
}

// ASRConfig represents speech recognition provider configuration.
type ASRConfig map[string]interface{}

type VoiceInfo struct {
	Name        string `yaml:"name"         json:"name"`         // Voice identifier
	Language    string `yaml:"language"     json:"language"`     // Language tag
	DisplayName string `yaml:"display_name" json:"display_name"` // Display name for UI
	Sex         string `yaml:"sex"          json:"sex"`          // Gender (male/female)
	Description string `yaml:"description"  json:"description"`  // Voice description
	AudioURL    string `yaml:"audio_url"    json:"audio_url"`    // Sample audio URL
}

// TTSConfig represents speech synthesis provider configuration.
type TTSConfig struct {
	Type            string      `yaml:"type"             json:"type"`             // TTS type
	Voice           string      `yaml:"voice"            json:"voice"`            // Voice identifier
	Format          string      `yaml:"format"           json:"format"`           // Output format
	OutputDir       string      `yaml:"output_dir"       json:"output_dir"`       // Output directory
	AppID           string      `yaml:"appid"            json:"appid"`            // Application ID
	Token           string      `yaml:"token"            json:"token"`            // API Token / Secret
	Cluster         string      `yaml:"cluster"          json:"cluster"`          // Cluster endpoint
	SupportedVoices []VoiceInfo `yaml:"supported_voices" json:"supported_voices"` // List of supported voices
}

// LLMConfig represents large language model provider configuration.
type LLMConfig struct {
	Type        string                 `yaml:"type"        json:"type"`        // LLM type
	ModelName   string                 `yaml:"model_name"  json:"model_name"`  // Model name
	BaseURL     string                 `yaml:"url"         json:"url"`         // API Base URL
	APIKey      string                 `yaml:"api_key"     json:"api_key"`     // API Key
	Temperature float64                `yaml:"temperature" json:"temperature"` // Temperature
	MaxTokens   int                    `yaml:"max_tokens"  json:"max_tokens"`  // Max tokens
	TopP        float64                `yaml:"top_p"       json:"top_p"`       // Top P
	Extra       map[string]interface{} `yaml:",inline"     json:"extra"`       // Extra parameters
}

// SecurityConfig represents image security scanning settings.
type SecurityConfig struct {
	MaxFileSize       int64    `yaml:"max_file_size"      json:"max_file_size"`      // Max file size in bytes
	MaxPixels         int64    `yaml:"max_pixels"         json:"max_pixels"`         // Max pixels
	MaxWidth          int      `yaml:"max_width"          json:"max_width"`          // Max width
	MaxHeight         int      `yaml:"max_height"         json:"max_height"`         // Max height
	AllowedFormats    []string `yaml:"allowed_formats"    json:"allowed_formats"`    // Allowed image formats
	EnableDeepScan    bool     `yaml:"enable_deep_scan"   json:"enable_deep_scan"`   // Enable deep security scan
	ValidationTimeout string   `yaml:"validation_timeout" json:"validation_timeout"` // Timeout duration
}

// VLLMConfig represents vision language model provider configuration.
type VLLMConfig struct {
	Type        string                 `yaml:"type"        json:"type"`        // API type
	ModelName   string                 `yaml:"model_name"  json:"model_name"`  // Vision model name
	BaseURL     string                 `yaml:"url"         json:"url"`         // API Base URL
	APIKey      string                 `yaml:"api_key"     json:"api_key"`     // API Key
	Temperature float64                `yaml:"temperature" json:"temperature"` // Temperature
	MaxTokens   int                    `yaml:"max_tokens"  json:"max_tokens"`  // Max tokens
	TopP        float64                `yaml:"top_p"       json:"top_p"`       // Top P
	Security    SecurityConfig         `yaml:"security"    json:"security"`    // Security settings
	Extra       map[string]interface{} `yaml:",inline"     json:"extra"`       // Extra parameters
}

var (
	Cfg *Config
)

func (cfg *Config) ToString() string {
	data, _ := yaml.Marshal(cfg)
	return string(data)
}

func (cfg *Config) FromString(data string) error {
	return yaml.Unmarshal([]byte(data), cfg)
}

func (cfg *Config) SaveToDB(dbi ConfigDBInterface) error {
	data := cfg.ToString()
	return dbi.UpdateServerConfig(data)
}

// LoadConfig loads the server configuration.
func LoadConfig(dbi ConfigDBInterface) (*Config, string, error) {
	bUseDatabaseCfg := true
	cfgStr, err := dbi.LoadServerConfig()
	if err != nil {
		fmt.Println("Failed to load server configuration from DB:", err)
		return nil, "", err
	}

	config := &Config{}

	path := "database:serverConfig"
	if cfgStr != "" {
		config.FromString(cfgStr)
		LoadProvidersFromDB(dbi, config)
		config = CheckAndModifyConfig(config)
		Cfg = config
		if bUseDatabaseCfg {
			return Cfg, path, nil
		}
	}

	// First start initializes private .config.yaml
	path = ".config.yaml"
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data, err = os.ReadFile("config.yaml")
		if errors.Is(err, os.ErrNotExist) {
			config = NewDefaultInitConfig()
			data, err = yaml.Marshal(config)
		}
		if err != nil {
			return nil, path, fmt.Errorf("failed to read initial configuration: %w", err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return nil, path, fmt.Errorf("failed to create private configuration file: %w", err)
		}
		return nil, path, fmt.Errorf("%w at %s; please configure model services before restarting the server", ErrInitialConfigCreated, path)
	} else if err != nil {
		return nil, path, fmt.Errorf("failed to read private configuration file: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, path, err
	}

	err = dbi.InitServerConfig(string(data))
	if err != nil {
		fmt.Println("Failed to initialize server configuration in database:", err)
	}
	config = CheckAndModifyConfig(config)
	Cfg = config
	return config, path, nil
}

func LoadProvidersFromDB(dbi ConfigDBInterface, cfg *Config) {
	// Load ASR providers
	asrData := dbi.LoadProviderData("ASR", 0)
	if asrData != nil {
		cfg.ASR = make(map[string]ASRConfig)
		for name, dataStr := range asrData {
			var asrConfig ASRConfig
			if err := yaml.Unmarshal([]byte(dataStr), &asrConfig); err == nil {
				cfg.ASR[name] = asrConfig
			}
		}
	}
	// Load TTS providers
	ttsData := dbi.LoadProviderData("TTS", 0)
	if ttsData != nil {
		cfg.TTS = make(map[string]TTSConfig)
		for name, dataStr := range ttsData {
			var ttsConfig TTSConfig
			if err := yaml.Unmarshal([]byte(dataStr), &ttsConfig); err == nil {
				cfg.TTS[name] = ttsConfig
			}
		}
	}
	// Load LLM providers
	llmData := dbi.LoadProviderData("LLM", 0)
	if llmData != nil {
		cfg.LLM = make(map[string]LLMConfig)
		for name, dataStr := range llmData {
			var llmConfig LLMConfig
			if err := yaml.Unmarshal([]byte(dataStr), &llmConfig); err == nil {
				cfg.LLM[name] = llmConfig
			}
		}
	}
	// Load VLLM providers
	vllmData := dbi.LoadProviderData("VLLLM", 0)
	if vllmData != nil {
		cfg.VLLLM = make(map[string]VLLMConfig)
		for name, dataStr := range vllmData {
			var vllmConfig VLLMConfig
			if err := yaml.Unmarshal([]byte(dataStr), &vllmConfig); err == nil {
				cfg.VLLLM[name] = vllmConfig
			}
		}
	}
	dbi.UpdateServerConfig(cfg.ToString())
}

func CheckAndModifyConfig(cfg *Config) *Config {
	if cfg.LocalMCPFun == nil {
		cfg.LocalMCPFun = []LocalMCPFun{}
	}
	fmt.Printf("Config Check: LocalMCPFun count %d\n", len(cfg.LocalMCPFun))
	if len(cfg.LocalMCPFun) < 10 {
		for i := 0; i < len(cfg.LocalMCPFun); i++ {
			cfg.LocalMCPFun[i].Name = strings.ToLower(strings.TrimSpace(cfg.LocalMCPFun[i].Name))
			cfg.LocalMCPFun[i].Description = strings.ToLower(strings.TrimSpace(cfg.LocalMCPFun[i].Description))
		}
	}
	if cfg.SelectedModule == nil {
		cfg.SelectedModule = map[string]string{}
	}
	if cfg.LLM == nil {
		cfg.LLM = map[string]LLMConfig{}
	}
	if cfg.VLLLM == nil {
		cfg.VLLLM = map[string]VLLMConfig{}
	}
	if cfg.ASR == nil {
		cfg.ASR = map[string]ASRConfig{}
	}
	if cfg.TTS == nil {
		cfg.TTS = map[string]TTSConfig{}
	}
	fmt.Printf("Config Check: LLM:%d, VLLLM:%d, ASR:%d, TTS:%d\n", len(cfg.LLM), len(cfg.VLLLM), len(cfg.ASR), len(cfg.TTS))
	fmt.Println("Config Check: SelectedModule", cfg.SelectedModule)

	llmName, ok := cfg.SelectedModule["LLM"]
	_, exists := cfg.LLM[llmName]
	if !ok || llmName == "" || !exists {
		for name := range cfg.LLM {
			cfg.SelectedModule["LLM"] = name
			fmt.Println("Default LLM unset or not found; assigned to", name)
			break
		}
	}
	defaulCfg := NewDefaultInitConfig()
	if len(cfg.LLM) == 0 {
		fmt.Println("Warning: No LLM provider found, using default configuration!")
		cfg.LLM = defaulCfg.LLM
		for name := range cfg.LLM {
			cfg.SelectedModule["LLM"] = name
			fmt.Println("Set default LLM to", name)
			break
		}
	}

	vlllmName, ok := cfg.SelectedModule["VLLLM"]
	_, exists = cfg.VLLLM[vlllmName]
	if !ok || vlllmName == "" || !exists {
		for name := range cfg.VLLLM {
			cfg.SelectedModule["VLLLM"] = name
			fmt.Println("Default VLLM unset or not found; assigned to", name)
			break
		}
	}
	if len(cfg.VLLLM) == 0 {
		fmt.Println("Warning: No VLLM provider found, using default configuration!")
		cfg.VLLLM = defaulCfg.VLLLM
		for name := range cfg.VLLLM {
			cfg.SelectedModule["VLLLM"] = name
			fmt.Println("Set default VLLM to", name)
			break
		}
	}

	asrName, ok := cfg.SelectedModule["ASR"]
	_, exists = cfg.ASR[asrName]
	if !ok || asrName == "" || !exists {
		for name := range cfg.ASR {
			cfg.SelectedModule["ASR"] = name
			fmt.Println("Default ASR unset or not found; assigned to", name)
			break
		}
	}

	if len(cfg.ASR) == 0 {
		fmt.Println("Warning: No ASR provider found, using default configuration!")
		cfg.ASR = defaulCfg.ASR
		for name := range cfg.ASR {
			cfg.SelectedModule["ASR"] = name
			fmt.Println("Set default ASR to", name)
			break
		}
	}

	ttsName, ok := cfg.SelectedModule["TTS"]
	_, exists = cfg.TTS[ttsName]
	if !ok || ttsName == "" || !exists {
		for name := range cfg.TTS {
			cfg.SelectedModule["TTS"] = name
			fmt.Println("Default TTS unset or not found; assigned to", name)
			break
		}
	}

	if len(cfg.TTS) == 0 {
		fmt.Println("Warning: No TTS provider found, using default configuration!")
		cfg.TTS = defaulCfg.TTS
		for name := range cfg.TTS {
			cfg.SelectedModule["TTS"] = name
			fmt.Println("Set default TTS to", name)
			break
		}
	}

	return cfg
}
