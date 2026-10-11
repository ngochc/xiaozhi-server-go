package configs

var DefaultCfg *Config

func (cfg *Config) setDefaults() {
	cfg.Transport.WebSocket.Enabled = true
	cfg.Transport.WebSocket.IP = "0.0.0.0"
	cfg.Transport.WebSocket.Port = 8000

	cfg.Transport.MQTTUDP.Enabled = false
	cfg.Transport.MQTTUDP.MQTT.IP = "your_ip_or_domain:1883"
	cfg.Transport.MQTTUDP.MQTT.Port = 1883
	cfg.Transport.MQTTUDP.MQTT.QoS = 1

	cfg.Transport.MQTTUDP.UDP.IP = "your_ip_or_domain"
	cfg.Transport.MQTTUDP.UDP.Port = 8100
	cfg.Transport.MQTTUDP.UDP.ShowPort = 8100

	cfg.Web.Port = 8080
	cfg.Web.Websocket = "ws://your_ip:8000"
	cfg.Web.VisionURL = "http://your_ip:8080/api/vision"
	cfg.Web.ActivateText = "Anime AI Chat"

	cfg.Server.Token = "your_token"
	cfg.Server.Auth.Store.Type = "database"
	cfg.Server.Auth.Store.Expiry = 24

	cfg.Log.LogDir = "logs"
	cfg.Log.LogLevel = "INFO"
	cfg.Log.LogFile = "server.log"
}

func NewDefaultInitConfig() *Config {
	config := &Config{}
	config.setDefaults()

	// Set default module selections
	config.SelectedModule = map[string]string{
		"ASR":   "DoubaoASR",
		"TTS":   "EdgeTTS",
		"LLM":   "ChatGLMLLM",
		"VLLLM": "ChatGLMVLLM",
	}

	// Set default ASR configurations
	config.ASR = map[string]ASRConfig{
		"DoubaoASR": {
			"type":         "doubao",
			"appid":        "your_appid",
			"access_token": "your_access_token",
			"output_dir":   "tmp/",
		},
	}

	// Set default TTS configurations
	config.TTS = map[string]TTSConfig{
		"EdgeTTS": {
			Type:      "edge",
			Voice:     "en-US-AriaNeural",
			OutputDir: "tmp/",
			SupportedVoices: []VoiceInfo{
				{Name: "en-US-AriaNeural", DisplayName: "Aria", Sex: "female", Description: "Natural American English female voice"},
				{Name: "en-US-GuyNeural", DisplayName: "Guy", Sex: "male", Description: "Clear American English male voice"},
				{Name: "en-US-JennyNeural", DisplayName: "Jenny", Sex: "female", Description: "Warm American English female voice"},
				{Name: "zh-CN-XiaoxiaoNeural", DisplayName: "Xiaoxiao", Sex: "female", Description: "Clear Mandarin female voice"},
			},
		},
	}

	// Set default LLM configurations
	config.LLM = map[string]LLMConfig{
		"ChatGLMLLM": {
			Type:      "openai",
			ModelName: "glm-4-flash",
			BaseURL:   "https://open.bigmodel.cn/api/paas/v4/",
			APIKey:    "your_api_key",
		},
	}

	// Set default VLLM configurations
	config.VLLLM = map[string]VLLMConfig{
		"ChatGLMVLLM": {
			Type:        "openai",
			ModelName:   "glm-4v-flash",
			BaseURL:     "https://open.bigmodel.cn/api/paas/v4/",
			APIKey:      "your_api_key",
			MaxTokens:   4096,
			Temperature: 0.7,
			TopP:        0.9,
			Security: SecurityConfig{
				MaxFileSize:       10485760,
				MaxPixels:         16777216,
				MaxWidth:          4096,
				MaxHeight:         4096,
				AllowedFormats:    []string{"jpeg", "jpg", "png", "webp", "gif"},
				EnableDeepScan:    true,
				ValidationTimeout: "10s",
			},
		},
	}

	// Set remaining defaults
	config.DeleteAudio = false
	config.SaveTTSAudio = false
	config.SaveUserAudio = false
	config.QuickReply = true
	config.QuickReplyWords = []string{"I'm here", "Yes?", "Coming", "How can I help?"}
	config.LocalMCPFun = []LocalMCPFun{
		{Name: "time", Description: "Get current time"},
		{Name: "exit", Description: "Exit application"},
		{Name: "change_role", Description: "Switch persona"},
		{Name: "play_music", Description: "Play music"},
		{Name: "change_voice", Description: "Change voice"},
	}
	config.DefaultPrompt = `You are Xiaozhi, a witty, friendly, and helpful voice assistant.
[Core Characteristics]
- Warm, lively, and conversational tone
- Speaks concisely and clearly, ideal for spoken audio
- Hidden enthusiasm for tech, programming, and creative ideas
Never:
- Give lengthy lectures or droning explanations
- Engage in overly rigid or robotic speech
- Include markdown formatting or emojis in spoken replies`
	config.Roles = []Role{
		{
			Name:        "English Tutor",
			Description: "I am Lily, your English conversation tutor. I speak fluent, natural American English. My mission is to help you practice daily spoken English using simple vocabulary and grammar. I keep responses short and engaging so you have plenty of time to talk and practice.",
		},
		{
			Name:        "Tech Companion",
			Description: "I am a smart and witty companion who loves discussing technology, programming, gadgets, and fun pop culture. I speak casually with a friendly and supportive tone.",
		},
		{
			Name:        "Curious Explorer",
			Description: "I am an 8-year-old curious explorer with a boundless passion for knowledge. From the vast universe to nature, science experiments, and world history, I love sharing exciting facts and solving mysteries together.",
		},
	}
	config.CMDExit = []string{"exit", "quit", "stop"}
	DefaultCfg = config
	return config
}
