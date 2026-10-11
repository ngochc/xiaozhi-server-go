package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/core/auth"
	"xiaozhi-server-go/src/core/chat"
	"xiaozhi-server-go/src/core/function"
	"xiaozhi-server-go/src/core/image"
	"xiaozhi-server-go/src/core/mcp"
	"xiaozhi-server-go/src/core/provider"
	"xiaozhi-server-go/src/core/providers"
	"xiaozhi-server-go/src/core/providers/llm"
	"xiaozhi-server-go/src/core/providers/tts"
	"xiaozhi-server-go/src/core/providers/vlllm"
	"xiaozhi-server-go/src/core/types"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/models"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
	"gorm.io/gorm"
)

type MCPResultHandler func(args interface{}) string

// Connection unified connection interface
type Connection interface {
	// Send message
	WriteMessage(messageType int, data []byte) error
	// Read message
	ReadMessage(stopChan <-chan struct{}) (messageType int, data []byte, err error)
	// Close connection
	Close() error
	// Get connection ID
	GetID() string
	// Get connection type
	GetType() string
	// Check connection status
	IsClosed() bool
	// Get last active time
	GetLastActiveTime() time.Time
	// Check if expired
	IsStale(timeout time.Duration) bool
}

type ttsConfigGetter interface {
	Config() *tts.Config
}

type llmConfigGetter interface {
	Config() *llm.Config
}

// ConnectionHandler connection handler structure
type ConnectionHandler struct {
	// Ensure AsrEventListener interface is implemented
	_           providers.AsrEventListener
	config      *configs.Config
	logger      *utils.Logger
	conn        Connection
	closeOnce   sync.Once
	authManager *auth.AuthManager // Authentication manager
	providers   struct {
		asr   providers.ASRProvider
		llm   providers.LLMProvider
		tts   providers.TTSProvider
		vlllm *vlllm.Provider // VLLLM provider, optional
	}

	initialVoice    string // Initial voice name
	ttsProviderName string // Default TTS provider name
	voiceName       string // Voice name

	// Session related
	sessionID     string            // Session ID
	deviceID      string            // Device ID
	clientId      string            // Client ID
	headers       map[string]string // HTTP headers
	transportType string            // Transport type

	// Client audio related
	clientAudioFormat        string
	clientAudioSampleRate    int
	clientAudioChannels      int
	clientAudioFrameDuration int

	serverAudioFormat        string // Server audio format
	serverAudioSampleRate    int
	serverAudioChannels      int
	serverAudioFrameDuration int

	clientListenMode string
	isDeviceVerified bool
	isDeviceDisabled bool
	closeAfterChat   bool

	// Agent related
	agentID      uint          // Bound Agent ID
	enabledTools []string      // Enabled tools list
	tools        []openai.Tool // Cached tools list
	// Speech processing related
	serverVoiceStop int32 // 1 if server voice stopped, stops sending audio

	opusDecoder *utils.OpusDecoder // Opus decoder

	// Dialogue related
	dialogueManager     *chat.DialogueManager
	tts_last_text_index int
	client_asr_text     string // Client ASR text
	quickReplyCache     *utils.QuickReplyCache

	// Concurrency control
	stopChan         chan struct{}
	clientAudioQueue chan []byte
	clientTextQueue  chan string

	// TTS task queue
	ttsQueue chan struct {
		text      string
		round     int // Round
		textIndex int
	}

	audioMessagesQueue chan struct {
		filepath  string
		text      string
		round     int // Round
		textIndex int
	}

	talkRound      int       // Round counter
	roundStartTime time.Time // Round start time
	// functions
	functionRegister *function.FunctionRegistry
	mcpManager       *mcp.Manager

	mcpResultHandlers map[string]MCPResultHandler // MCP result handlers mapping
	ctx               context.Context
}

// NewConnectionHandler creates a new connection handler
func NewConnectionHandler(
	config *configs.Config,
	providerSet *provider.ProviderSet,
	logger *utils.Logger,
	req *http.Request,
	ctx context.Context,
) *ConnectionHandler {
	handler := &ConnectionHandler{
		config:           config,
		logger:           logger,
		clientListenMode: "auto",
		stopChan:         make(chan struct{}),
		clientAudioQueue: make(chan []byte, 100),
		clientTextQueue:  make(chan string, 100),
		ttsQueue: make(chan struct {
			text      string
			round     int // Round
			textIndex int
		}, 100),
		audioMessagesQueue: make(chan struct {
			filepath  string
			text      string
			round     int // Round
			textIndex int
		}, 100),

		tts_last_text_index: -1,

		talkRound: 0,

		serverAudioFormat:        "opus", // Default Opus format
		serverAudioSampleRate:    24000,
		serverAudioChannels:      1,
		serverAudioFrameDuration: 60,

		ctx: ctx,

		headers: make(map[string]string),
	}

	for key, values := range req.Header {
		if len(values) > 0 {
			handler.headers[key] = values[0] // Use first value
		}
		if key == "Device-Id" {
			handler.deviceID = values[0] // Device ID
		}
		if key == "Client-Id" {
			handler.clientId = values[0] // Client ID
		}
		if key == "Session-Id" {
			handler.sessionID = values[0] // Session ID
		}
		if key == "Transport-Type" {
			handler.transportType = values[0] // Transport type
		}
		logger.Debug("[HTTP] [header %s] %s", key, values[0])
	}

	if handler.sessionID == "" {
		if handler.deviceID == "" {
			handler.sessionID = uuid.New().String() // Generate new session ID if missing
		} else {
			handler.sessionID = "device-" + strings.Replace(handler.deviceID, ":", "_", -1)
		}
	}

	// Set providers
	if providerSet != nil {
		handler.providers.asr = providerSet.ASR
		handler.providers.llm = providerSet.LLM
		handler.providers.tts = providerSet.TTS
		handler.providers.vlllm = providerSet.VLLLM
		handler.mcpManager = providerSet.MCP
	}
	handler.checkDeviceInfo()
	agent, prompt := handler.InitWithAgent()
	handler.checkTTSProvider(agent, config) // Check TTS provider
	handler.checkLLMProvider(agent, config) // Check LLM provider match

	handler.quickReplyCache = utils.NewQuickReplyCache(handler.ttsProviderName, handler.voiceName)

	// Initialize dialogue manager
	memory := chat.NewDeviceAgentShortTermMemory(handler.deviceID, handler.agentID)
	handler.dialogueManager = chat.NewDialogueManager(handler.logger, memory)
	handler.dialogueManager.SetShortTermMemoryLimits(chat.DefaultShortTermMemoryMessages, chat.DefaultShortTermMemoryTokens)
	handler.dialogueManager.SetSystemMessage(prompt)
	handler.functionRegister = function.NewFunctionRegistry()
	handler.initMCPResultHandlers()

	return handler
}

func (h *ConnectionHandler) InitWithAgent() (*models.Agent, string) {
	// Retrieve Agent by agentID
	var agent *models.Agent = nil
	var err error
	prompt := h.config.DefaultPrompt
	if h.agentID != 0 {
		// Transaction not required here
		agent, err = database.GetAgentByID(database.GetDB(), h.agentID)
		if err != nil {
			h.LogError(fmt.Sprintf("failed to get Agent: %v", err))
		}
		agentName := agent.Name
		prompt = agent.Prompt // Use Agent prompt
		if agentName != "" {
			if strings.Contains(prompt, "{{assistant_name}}") {
				prompt = strings.Replace(prompt, "{{assistant_name}}", agentName, -1)
			} else {
				prompt += "\n\nAssistant name: " + agentName
			}
		}

		if agent.Language != "" && agent.Language != "English" && agent.Language != "\u666e\u901a\u8bdd" && agent.Language != "\u4e2d\u6587" {
			prompt += "\n\nAnswer user questions in " + agent.Language + "."
		}

		if agent.EnabledTools != "" {
			h.enabledTools = strings.Split(agent.EnabledTools, ",")
		} else {
			h.enabledTools = []string{} // No filter if empty
		}

		h.LogInfo(fmt.Sprintf("allowed tools: %v", h.enabledTools))
		h.LogInfo(fmt.Sprintf("using prompt for Agent %d: %s", h.agentID, prompt))

	}
	return agent, prompt
}

func (h *ConnectionHandler) checkTTSProvider(agent *models.Agent, config *configs.Config) {
	h.ttsProviderName = "default" // Default TTS provider name
	h.voiceName = "default"
	if getter, ok := h.providers.tts.(ttsConfigGetter); ok {

		userID := database.AdminUserID
		alltts, err := database.GetProviderByTypeInternal("TTS", userID, false)
		if err == nil {
			for name, data := range alltts {

				cfg := configs.TTSConfig{}
				if err := json.Unmarshal([]byte(data), &cfg); err != nil {
					h.LogError(fmt.Sprintf("failed to deserialize user %d TTS provider %s config: %v", userID, name, err))
					continue
				}
				// h.LogInfo(fmt.Sprintf("User %d TTS provider: %s, config: %v", userID, name, cfg))
				config.TTS[name] = cfg // Update config
			}
		} else {
			h.LogError(fmt.Sprintf("failed to get user %d TTS provider: %v", userID, err))
		}

		h.ttsProviderName = getter.Config().Type
		// Retrieve from agent config
		h.voiceName = getter.Config().Voice
		if agent != nil && agent.Voice != "" {
			err, newVoice := h.providers.tts.SetVoice(agent.Voice) // Set TTS voice
			if err != nil {
				// Check if supported by other TTS providers
				bChangeTTSSucc := false
				for name, cfg := range config.TTS {
					if bSupport, newVoice2, _ := tts.IsSupportedVoice(agent.Voice, cfg.SupportedVoices); bSupport {
						ttsCfg := &tts.Config{
							Name:            name,
							Type:            cfg.Type,
							OutputDir:       cfg.OutputDir,
							Voice:           newVoice2,
							Format:          cfg.Format,
							SampleRate:      h.serverAudioSampleRate,
							AppID:           cfg.AppID,
							Token:           cfg.Token,
							Cluster:         cfg.Cluster,
							SupportedVoices: cfg.SupportedVoices,
						}
						newVoice = newVoice2
						newtts, err := tts.Create(cfg.Type, ttsCfg, false)
						if err == nil {
							h.providers.tts = newtts
							bChangeTTSSucc = true
							h.ttsProviderName = cfg.Type
							h.LogInfo(fmt.Sprintf("switched TTS provider to: %s, voice: %s, v:%s", name, agent.Voice, newVoice))
							break
						} else {
							h.LogError(fmt.Sprintf("failed to create TTS provider: %v", err))
						}
					} else {
						h.LogInfo(fmt.Sprintf("voice %s for Agent %d not supported in TTS provider %s", agent.Voice, agent.ID, name))
					}
				}
				if !bChangeTTSSucc {
					h.LogError(fmt.Sprintf("failed to set TTS voice to agent config: %v", err))
				} else {
					h.voiceName = newVoice
				}
			} else {
				h.voiceName = newVoice
			}
		}
		h.initialVoice = h.voiceName // Save initial voice name
	}
	h.logger.Info("using TTS provider: %s, voice: %s", h.ttsProviderName, h.voiceName)

}

func (h *ConnectionHandler) checkLLMProvider(agent *models.Agent, config *configs.Config) {
	if agent == nil {
		return
	}
	agentLLMName := agent.LLM
	// Get extra from agent
	apiKey := ""
	baseUrl := ""
	if agent.Extra != "" {
		// Parse Extra field
		var extra map[string]interface{}
		if err := json.Unmarshal([]byte(agent.Extra), &extra); err == nil {
			if key, ok := extra["api_key"].(string); ok {
				apiKey = key
			}
			if url, ok := extra["base_url"].(string); ok {
				baseUrl = url
			}
		} else {
			h.LogError(fmt.Sprintf("Agent %d Extra field format error: %v, err:%v", agent.ID, agent.Extra, err))
		}
	}
	// Check if handler.providers.llm type matches agent.LLM
	if getter, ok := h.providers.llm.(llmConfigGetter); ok {
		// Load user LLM config from database
		userID := database.AdminUserID
		llms, err := database.GetProviderByTypeInternal("LLM", userID, false)
		if err == nil {
			for name, data := range llms {

				cfg := configs.LLMConfig{}
				if err := json.Unmarshal([]byte(data), &cfg); err != nil {
					h.LogError(fmt.Sprintf("failed to deserialize user %d LLM provider %s config: %v", userID, name, err))
					continue
				}
				//h.LogInfo(fmt.Sprintf("User %d LLM provider: %s, config: %v", userID, name, cfg))
				config.LLM[name] = cfg // Update config
			}
		} else {
			h.LogError(fmt.Sprintf("failed to get user %d LLM provider: %v", userID, err))
		}

		llmName := getter.Config().Name
		if cfg, ok := config.LLM[agentLLMName]; !ok {
			h.LogError(fmt.Sprintf("Agent %d LLM type %s does not exist", h.agentID, agentLLMName))
		} else {
			if apiKey != "" {
				cfg.APIKey = apiKey // Use Agent API key
			}
			if baseUrl != "" {
				cfg.BaseURL = baseUrl // Use Agent BaseURL
			}
			current := getter.Config()
			// Rebuild instance to pick up new key/URL loaded from database
			needsRecreate := llmName != agentLLMName || current.Type != cfg.Type || current.ModelName != cfg.ModelName || current.BaseURL != cfg.BaseURL || current.APIKey != cfg.APIKey
			if needsRecreate {
				llmCfg := &llm.Config{
					Name:        agentLLMName,
					Type:        cfg.Type,
					ModelName:   cfg.ModelName,
					BaseURL:     cfg.BaseURL,
					APIKey:      cfg.APIKey,
					Temperature: cfg.Temperature,
					MaxTokens:   cfg.MaxTokens,
					TopP:        cfg.TopP,
					Extra:       cfg.Extra,
				}
				newllm, err := llm.Create(cfg.Type, llmCfg)
				if err != nil {
					h.LogError(fmt.Sprintf("failed to create LLM provider: %v", err))
				} else {
					h.providers.llm = newllm
					h.LogInfo(fmt.Sprintf("applied LLM provider for Agent %d: %s", h.agentID, agentLLMName))
				}
			}
			h.LogInfo(fmt.Sprintf("using LLM for Agent %d: type=%s, BaseURL=%s", h.agentID, agentLLMName, cfg.BaseURL))
		}
	}
}

func (h *ConnectionHandler) checkDeviceInfo() {
	h.agentID = 0 // Clear AgentID

	if h.deviceID == "" {
		h.LogError("Device ID not set, cannot check device binding status")
		return
	}
	device, err := database.FindDeviceByID(database.GetDB(), h.deviceID) // Ensure device exists
	if err == gorm.ErrRecordNotFound {
		h.LogError(fmt.Sprintf("failed to find device: %v", err))
		return
	}
	if device.Mode == "ban" {
		h.isDeviceDisabled = true
		h.LogError("device is disabled, rejecting session")
		return
	}

	if device.AgentID != nil {
		h.agentID = *device.AgentID // Get bound AgentID
	} else {
		onboardingAgent, ensureErr := database.GetOnboardingAgent(database.GetDB())
		if ensureErr == gorm.ErrRecordNotFound {
			h.LogInfo("onboarding assistant not yet created, device remains unbound")
		} else if ensureErr != nil {
			h.LogError(fmt.Sprintf("failed to query onboarding assistant: %v", ensureErr))
		} else {
			ownerID := onboardingAgent.UserID
			device.AgentID = &onboardingAgent.ID
			device.UserID = &ownerID
			device.AuthStatus = "onboarding"
			if err := database.UpdateDevice(database.GetDB(), device); err != nil {
				h.LogError(fmt.Sprintf("failed to save initial device binding: %v", err))
			} else {
				h.agentID = onboardingAgent.ID
				h.LogInfo("device automatically bound to onboarding assistant")
			}
		}
	}

	h.LogInfo(fmt.Sprintf("device binding status: AgentID=%d", h.agentID))
}

func (h *ConnectionHandler) LogInfo(msg string) {
	if h.logger != nil {
		h.logger.Info(msg, map[string]interface{}{
			"device": h.deviceID,
		})
	}
}
func (h *ConnectionHandler) LogDebug(msg string) {
	if h.logger != nil {
		h.logger.Debug(msg, map[string]interface{}{
			"device": h.deviceID,
		})
	}
}
func (h *ConnectionHandler) LogError(msg string) {
	if h.logger != nil {
		h.logger.Error(msg, map[string]interface{}{
			"device": h.deviceID,
		})
	}
}

// Handle handles WebSocket connection
func (h *ConnectionHandler) Handle(conn Connection) {
	defer conn.Close()
	defer h.dialogueManager.EndSession()

	h.conn = conn
	if h.isDeviceDisabled {
		h.LogError("device is disabled, connection closed")
		return
	}

	// Launch message handling goroutines
	go h.processClientAudioMessagesCoroutine() // Client audio handling goroutine
	go h.processClientTextMessagesCoroutine()  // Client text handling goroutine
	go h.processTTSQueueCoroutine()            // TTS queue handling goroutine
	go h.sendAudioMessageCoroutine()           // Audio sending goroutine

	// MCP manager handling
	if h.mcpManager == nil {
		h.LogError("no available MCP manager")
		return

	} else {
		h.LogInfo("[MCP] [Manager] binding connection-level MCP manager")
		// Pre-initialized manager only needs connection binding
		params := map[string]interface{}{
			"session_id": h.sessionID,
			"vision_url": h.config.Web.VisionURL,
			"device_id":  h.deviceID,
			"client_id":  h.clientId,
			"token":      h.config.Server.Token,
		}
		if err := h.mcpManager.BindConnection(conn, h.functionRegister, params); err != nil {
			h.LogError(fmt.Sprintf("failed to bind MCP manager connection: %v", err))
			return
		}
		// Skip reinitialization, ensure connection services are active
		h.LogInfo("[MCP] [Bind] connection bind completed, skipping duplicate initialization")
	}

	// Main message loop
	for {
		select {
		case <-h.stopChan:
			return
		default:
			messageType, message, err := conn.ReadMessage(h.stopChan)
			if err != nil {
				h.LogError(fmt.Sprintf("failed to read message: %v, exiting main loop", err))
				return
			}

			if err := h.handleMessage(messageType, message); err != nil {
				h.LogError(fmt.Sprintf("failed to process message: %v", err))
			}
		}
	}
}

// processClientTextMessagesCoroutine handles client text messages queue
func (h *ConnectionHandler) processClientTextMessagesCoroutine() {
	for {
		select {
		case <-h.stopChan:
			return
		case text := <-h.clientTextQueue:
			if err := h.processClientTextMessage(context.Background(), text); err != nil {
				h.LogError(fmt.Sprintf("failed to process text data: %v", err))
			}
		}
	}
}

// processClientAudioMessagesCoroutine handles client audio messages queue
func (h *ConnectionHandler) processClientAudioMessagesCoroutine() {
	for {
		select {
		case <-h.stopChan:
			return
		case audioData := <-h.clientAudioQueue:
			if h.closeAfterChat {
				continue
			}
			if err := h.providers.asr.AddAudio(audioData); err != nil {
				h.LogError(fmt.Sprintf("failed to process audio data: %v", err))
			}
		}
	}
}

func (h *ConnectionHandler) sendAudioMessageCoroutine() {
	for {
		select {
		case <-h.stopChan:
			return
		case task := <-h.audioMessagesQueue:
			h.sendAudioMessage(task.filepath, task.text, task.textIndex, task.round)
		}
	}
}

// OnAsrResult implements AsrEventListener interface
// Returns true to stop speech recognition, false to continue
func (h *ConnectionHandler) OnAsrResult(result string, isFinalResult bool) bool {
	//h.LogInfo(fmt.Sprintf("[%s] ASR recognition result: %s", h.clientListenMode, result))
	if h.providers.asr.GetSilenceCount() >= 2 {
		h.LogInfo("[ASR] [Silence Detection] two consecutive silences, ending dialogue")
		h.closeAfterChat = true // End dialogue after two consecutive silences
		result = "[SILENCE_TIMEOUT] No user speech detected for a prolonged period. Please politely end the conversation."
	}
	if h.clientListenMode == "auto" {
		if result == "" {
			return false
		}
		h.LogInfo(fmt.Sprintf("[ASR] [result %s/%s]", h.clientListenMode, result))
		h.handleChatMessage(context.Background(), result)
		return true
	} else if h.clientListenMode == "manual" {
		h.client_asr_text += result
		if isFinalResult {
			h.handleChatMessage(context.Background(), h.client_asr_text)
			return true
		}
		return false
	} else if h.clientListenMode == "realtime" {
		if result == "" {
			return false
		}
		h.stopServerSpeak()
		h.providers.asr.Reset() // Reset ASR state for next round
		h.LogInfo(fmt.Sprintf("[ASR] [result %s/%s]", h.clientListenMode, result))
		h.handleChatMessage(context.Background(), result)
		return true
	}
	return false
}

// clientAbortChat handles abort message
func (h *ConnectionHandler) clientAbortChat() error {
	h.LogInfo("[Client] [abort message] received, stopping speech recognition")
	h.stopServerSpeak()
	h.sendTTSMessage("stop", "", 0)
	h.clearSpeakStatus()
	return nil
}

func (h *ConnectionHandler) QuitIntent(text string) bool {
	// CMD_exit read exit commands from config
	exitCommands := h.config.CMDExit
	if exitCommands == nil {
		return false
	}
	cleand_text := utils.RemoveAllPunctuation(text) // Remove punctuation for matching
	// Check if contains exit command
	for _, cmd := range exitCommands {
		h.logger.Debug(fmt.Sprintf("check exit command: %s,%s", cmd, cleand_text))
		// Check equality
		if cleand_text == cmd {
			h.LogInfo("[Client] [exit intent] received, preparing to end dialogue")
			h.Close() // Close connection directly
			return true
		}
	}
	return false
}

func (h *ConnectionHandler) quickReplyWakeUpWords(text string) bool {
	// Check if wakeup word
	if !h.config.QuickReply || h.talkRound != 1 {
		return false
	}
	if !utils.IsWakeUpWord(text) {
		return false
	}

	repalyWords := h.config.QuickReplyWords
	reply_text := utils.RandomSelectFromArray(repalyWords)
	h.tts_last_text_index = 1 // Reset text index
	h.SpeakAndPlay(reply_text, 1, h.talkRound)

	return true
}

// handleChatMessage handles chat message
func (h *ConnectionHandler) handleChatMessage(ctx context.Context, text string) error {
	if text == "" {
		h.logger.Warn("received empty chat message, ignoring")
		h.clientAbortChat()
		return fmt.Errorf("chat message is empty")
	}

	if h.QuitIntent(text) {
		return fmt.Errorf("user requested to exit dialogue")
	}

	// Increment dialogue round
	h.talkRound++
	h.roundStartTime = time.Now()
	currentRound := h.talkRound
	h.LogInfo(fmt.Sprintf("[Dialogue] [round %d] starting new dialogue round", currentRound))

	// Standard text message processing
	// Send STT message immediately
	err := h.sendSTTMessage(text)
	if err != nil {
		h.LogError(fmt.Sprintf("failed to send STT message: %v", err))
		return fmt.Errorf("failed to send STT message: %v", err)
	}

	// Send TTS start state
	if err := h.sendTTSMessage("start", "", 0); err != nil {
		h.LogError(fmt.Sprintf("failed to send TTS start state: %v", err))
		return fmt.Errorf("failed to send TTS start state: %v", err)
	}

	// Send thinking emotion
	if err := h.sendEmotionMessage("thinking"); err != nil {
		h.LogError(fmt.Sprintf("failed to send thinking emotion message: %v", err))
		return fmt.Errorf("failed to send emotion message: %v", err)
	}

	h.LogInfo(fmt.Sprintf("[Chat] [message %s]", text))

	if h.quickReplyWakeUpWords(text) {
		return nil
	}

	// Add user message to dialogue history
	h.dialogueManager.Put(chat.Message{
		Role:    "user",
		Content: text,
	})

	return h.genResponseByLLM(ctx, h.dialogueManager.GetLLMDialogue(), currentRound)
}

func (h *ConnectionHandler) genResponseByLLM(ctx context.Context, messages []providers.Message, round int) error {
	defer func() {
		if r := recover(); r != nil {
			h.LogError(fmt.Sprintf("panic in genResponseByLLM: %v", r))
			errorMsg := "Sorry, an error occurred while processing your request"
			h.tts_last_text_index = 1 // Reset text index
			h.SpeakAndPlay(errorMsg, 1, round)
		}
	}()

	llmStartTime := time.Now()
	//h.logger.Info("starting LLM reply generation, round: %d", round)
	for _, msg := range messages {
		_ = msg
		//msg.Print()
	}
	// Generate reply using LLM
	tools := h.functionRegister.GetAllFunctions()
	responses, err := h.providers.llm.ResponseWithFunctions(ctx, h.sessionID, messages, tools)
	if err != nil {
		return fmt.Errorf("LLM failed to generate reply: %v", err)
	}

	// Process response
	var responseMessage []string
	processedChars := 0
	textIndex := 0

	atomic.StoreInt32(&h.serverVoiceStop, 0)

	// Process streaming response
	toolCallFlag := false
	functionName := ""
	functionID := ""
	functionArguments := ""
	contentArguments := ""

	for response := range responses {
		content := response.Content
		toolCall := response.ToolCalls

		if response.Error != "" {
			h.LogError(fmt.Sprintf("LLM response error: %s", response.Error))
			errorMsg := "Sorry, the service is temporarily unavailable. Please try again later."
			h.tts_last_text_index = 1 // Reset text index
			h.SpeakAndPlay(errorMsg, 1, round)
			return fmt.Errorf("LLM response error: %s", response.Error)
		}

		if content != "" {
			// Accumulate content_arguments
			contentArguments += content
		}

		if !toolCallFlag && strings.HasPrefix(contentArguments, "<tool_call>") {
			toolCallFlag = true
		}

		if len(toolCall) > 0 {
			toolCallFlag = true
			if toolCall[0].ID != "" {
				functionID = toolCall[0].ID
			}
			if toolCall[0].Function.Name != "" {
				functionName = toolCall[0].Function.Name
			}
			if toolCall[0].Function.Arguments != "" {
				functionArguments += toolCall[0].Function.Arguments
			}
		}

		if content != "" {
			if strings.Contains(content, "service response error") || strings.Contains(content, "\u670d\u52a1\u54cd\u5e94\u5f02\u5e38") {
				h.LogError(fmt.Sprintf("detected LLM service error: %s", content))
				errorMsg := "Sorry, the LLM service is temporarily unavailable. Please try again later."
				h.tts_last_text_index = 1 // Reset text index
				h.SpeakAndPlay(errorMsg, 1, round)
				return fmt.Errorf("LLM service error")
			}

			if toolCallFlag {
				continue
			}

			responseMessage = append(responseMessage, content)
			// Process segmentation
			fullText := utils.JoinStrings(responseMessage)
			if len(fullText) <= processedChars {
				h.logger.Warn(fmt.Sprintf("text processing anomaly: fullText len=%d, processedChars=%d", len(fullText), processedChars))
				continue
			}
			currentText := fullText[processedChars:]

			// Split by punctuation
			if segment, charsCnt := utils.SplitAtLastPunctuation(currentText); charsCnt > 0 {
				textIndex++
				segment = strings.TrimSpace(segment)
				if textIndex == 1 {
					now := time.Now()
					llmSpentTime := now.Sub(llmStartTime)
					h.LogInfo(fmt.Sprintf("[LLM] [reply %s/%d] first sentence: %s", llmSpentTime, round, segment))
				} else {
					h.LogInfo(fmt.Sprintf("[LLM] [segment %d/%d] %s", textIndex, round, segment))
				}
				h.tts_last_text_index = textIndex
				err := h.SpeakAndPlay(segment, textIndex, round)
				if err != nil {
					h.LogError(fmt.Sprintf("failed to play LLM reply segment: %v", err))
				}
				processedChars += charsCnt
			}
		}
	}

	if toolCallFlag {
		bHasError := false
		if functionID == "" {
			a := utils.Extract_json_from_string(contentArguments)
			if a != nil {
				functionName = a["name"].(string)
				argumentsJson, err := json.Marshal(a["arguments"])
				if err != nil {
					h.LogError(fmt.Sprintf("failed to parse function arguments: %v", err))
				}
				functionArguments = string(argumentsJson)
				functionID = uuid.New().String()
			} else {
				bHasError = true
			}
			if bHasError {
				h.LogError(fmt.Sprintf("failed to parse function arguments: %v", err))
			}
		}
		if !bHasError {
			// Clear responseMessage
			responseMessage = []string{}
			arguments := make(map[string]interface{})
			if err := json.Unmarshal([]byte(functionArguments), &arguments); err != nil {
				h.LogError(fmt.Sprintf("failed to parse function arguments: %v", err))
			}
			functionCallData := map[string]interface{}{
				"id":        functionID,
				"name":      functionName,
				"arguments": functionArguments,
			}
			h.LogInfo(fmt.Sprintf("function call: %v", arguments))
			if h.mcpManager.IsMCPTool(functionName) {
				// Handle MCP function call
				result, err := h.mcpManager.ExecuteTool(ctx, functionName, arguments)
				if err != nil {
					h.LogError(fmt.Sprintf("MCP function call failed: %v", err))
					if result == nil {
						result = "MCP tool call failed"
					}
				}
				// Check if result is types.ActionResponse type
				if actionResult, ok := result.(types.ActionResponse); ok {
					h.handleFunctionResult(actionResult, functionCallData, textIndex)
				} else {
					h.LogInfo(fmt.Sprintf("MCP function call result: %v", result))
					actionResult := types.ActionResponse{
						Action: types.ActionTypeReqLLM, // Action type
						Result: result,                 // Action result
					}
					h.handleFunctionResult(actionResult, functionCallData, textIndex)
				}

			} else {
				// Handle standard function call
				//h.functionRegister.CallFunction(functionName, functionCallData)
			}
		}
	}

	// Process remaining text
	fullResponse := utils.JoinStrings(responseMessage)
	if len(fullResponse) > processedChars {
		remainingText := fullResponse[processedChars:]
		if remainingText != "" {
			textIndex++
			h.LogInfo(fmt.Sprintf("[LLM] [remaining segment %d/%d] %s", textIndex, round, remainingText))
			h.tts_last_text_index = textIndex
			h.SpeakAndPlay(remainingText, textIndex, round)
		}
	} else {
		h.logger.Debug("no remaining text to process: fullResponse len=%d, processedChars=%d", len(fullResponse), processedChars)
	}

	// Analyze reply and send emotion
	content := utils.JoinStrings(responseMessage)

	// Add assistant reply to dialogue history
	if !toolCallFlag {
		h.dialogueManager.Put(chat.Message{
			Role:    "assistant",
			Content: content,
		})
	}

	return nil
}

func (h *ConnectionHandler) addToolCallMessage(toolResultText string, functionCallData map[string]interface{}) {

	functionID := functionCallData["id"].(string)
	functionName := functionCallData["name"].(string)
	functionArguments := functionCallData["arguments"].(string)
	h.LogInfo(fmt.Sprintf("tool call result: %s", toolResultText))
	h.LogInfo(fmt.Sprintf("tool call arguments: %s", functionArguments))
	h.LogInfo(fmt.Sprintf("tool call function: %s", functionName))
	h.LogInfo(fmt.Sprintf("tool call ID: %s", functionID))

	// Add assistant message with tool_calls
	h.dialogueManager.Put(chat.Message{
		Role: "assistant",
		ToolCalls: []types.ToolCall{{
			ID: functionID,
			Function: types.FunctionCall{
				Arguments: functionArguments,
				Name:      functionName,
			},
			Type:  "function",
			Index: 0,
		}},
	})

	// Add tool message
	toolCallID := functionID
	if toolCallID == "" {
		toolCallID = uuid.New().String()
	}
	h.dialogueManager.Put(chat.Message{
		Role:       "tool",
		ToolCallID: toolCallID,
		Content:    toolResultText,
	})
}

func (h *ConnectionHandler) handleFunctionResult(result types.ActionResponse, functionCallData map[string]interface{}, textIndex int) {
	switch result.Action {
	case types.ActionTypeError:
		h.LogError(fmt.Sprintf("function call error: %v", result.Result))
	case types.ActionTypeNotFound:
		h.LogError(fmt.Sprintf("function not found: %v", result.Result))
	case types.ActionTypeNone:
		h.LogInfo(fmt.Sprintf("function call no action: %v", result.Result))
	case types.ActionTypeResponse:
		h.LogInfo(fmt.Sprintf("function call direct response: %v", result.Response))
		h.SystemSpeak(result.Response.(string))
	case types.ActionTypeCallHandler:
		resultStr := h.handleMCPResultCall(result)
		h.addToolCallMessage(resultStr, functionCallData)
	case types.ActionTypeReqLLM:
		h.LogInfo(fmt.Sprintf("function call request LLM: %v", result.Result))
		text, ok := result.Result.(string)
		if ok && len(text) > 0 {
			h.addToolCallMessage(text, functionCallData)
			h.genResponseByLLM(context.Background(), h.dialogueManager.GetLLMDialogue(), h.talkRound)

		} else {
			h.LogError(fmt.Sprintf("failed to parse function call result: %v", result.Result))
			// Send error message
			errorMessage := fmt.Sprintf("failed to parse function call result: %v", result.Result)
			h.SystemSpeak(errorMessage)
		}
	}
}

func (h *ConnectionHandler) SystemSpeak(text string) error {
	if text == "" {
		h.logger.Warn("SystemSpeak received empty text, cannot synthesize speech")
		return errors.New("received empty text, cannot synthesize speech")
	}
	texts := utils.SplitByPunctuation(text)
	index := 0
	for _, item := range texts {
		index++
		h.tts_last_text_index = index // Reset text index
		h.SpeakAndPlay(item, index, h.talkRound)
	}
	return nil
}

// isNeedAuth checks if authentication is needed
func (h *ConnectionHandler) isNeedAuth() bool {
	return !h.isDeviceVerified
}

// processTTSQueueCoroutine processes TTS queue
func (h *ConnectionHandler) processTTSQueueCoroutine() {
	for {
		select {
		case <-h.stopChan:
			return
		case task := <-h.ttsQueue:
			h.processTTSTask(task.text, task.textIndex, task.round)
		}
	}
}

// Server barges in / stops speaking
func (h *ConnectionHandler) stopServerSpeak() {
	h.LogInfo("[Server] [voice] stopped speaking")
	atomic.StoreInt32(&h.serverVoiceStop, 1)
	h.cleanTTSAndAudioQueue(false)
}

func (h *ConnectionHandler) deleteAudioFileIfNeeded(filepath string, reason string) {
	if !h.config.DeleteAudio || filepath == "" {
		return
	}

	// Do not delete if quick reply cached file
	if h.quickReplyCache != nil && h.quickReplyCache.IsCachedFile(filepath) {
		h.LogInfo(fmt.Sprintf(reason+" skipping deletion of cached audio file: %s", filepath))
		return
	}

	// Do not delete if music file
	if utils.IsMusicFile(filepath) {
		h.LogInfo(fmt.Sprintf(reason+" skipping deletion of music file: %s", filepath))
		return
	}

	// Delete non-cached audio file
	if err := os.Remove(filepath); err != nil {
		h.LogError(fmt.Sprintf(reason+" failed to delete audio file: %v", err))
	} else {
		h.logger.Debug(fmt.Sprintf(reason+" deleted audio file: %s", filepath))
	}
}

// processTTSTask processes single TTS task
func (h *ConnectionHandler) processTTSTask(text string, textIndex int, round int) {
	filepath := ""
	defer func() {
		h.audioMessagesQueue <- struct {
			filepath  string
			text      string
			round     int
			textIndex int
		}{filepath, text, round, textIndex}
	}()

	if utils.IsQuickReplyHit(text, h.config.QuickReplyWords) {
		// Attempt to find audio file in cache
		if cachedFile := h.quickReplyCache.FindCachedAudio(text); cachedFile != "" {
			h.LogInfo(fmt.Sprintf("[TTS] [cache] using quick reply audio file=%s", cachedFile))
			filepath = cachedFile
			return
		}
	}
	ttsStartTime := time.Now()
	// Filter emojis
	text = utils.RemoveAllEmoji(text)
	// Remove parentheses and content
	text = utils.RemoveParentheses(text)

	if text == "" {
		h.logger.Warn(fmt.Sprintf("[TTS] [warning] received empty text index=%d", textIndex))
		return
	}

	// Generate speech file
	filepath, err := h.providers.tts.ToTTS(text)
	if err != nil {
		h.LogError(fmt.Sprintf("TTS conversion failed: text(%s) %v", text, err))
		return
	} else {
		h.logger.Debug(fmt.Sprintf("TTS conversion succeeded: text(%s), index(%d) %s", text, textIndex, filepath))
		// Save quick reply phrase to cache
		if utils.IsQuickReplyHit(text, h.config.QuickReplyWords) {
			if err := h.quickReplyCache.SaveCachedAudio(text, filepath); err != nil {
				h.LogError(fmt.Sprintf("failed to save quick reply audio: %v", err))
			} else {
				h.LogInfo(fmt.Sprintf("[TTS] [cache] cached quick reply audio text=%s", text))
			}
		}
	}
	if atomic.LoadInt32(&h.serverVoiceStop) == 1 { // Server voice stopped
		h.LogInfo(fmt.Sprintf("processTTSTask: server voice stopped, not sending audio data: %s", text))
		// Delete generated audio file on server voice stop if configured
		h.deleteAudioFileIfNeeded(filepath, "server voice stopped")
		return
	}

	if textIndex == 1 {
		now := time.Now()
		ttsSpentTime := now.Sub(ttsStartTime)
		h.logger.Debug(fmt.Sprintf("TTS conversion elapsed: %s, text: %s, index: %d", ttsSpentTime, text, textIndex))
	}

}

// speakAndPlay synthesizes and plays speech
func (h *ConnectionHandler) SpeakAndPlay(text string, textIndex int, round int) error {
	defer func() {
		// Enqueue task to prevent blocking
		h.ttsQueue <- struct {
			text      string
			round     int
			textIndex int
		}{text, round, textIndex}
	}()

	originText := text // Save original text for logging
	text = utils.RemoveAllEmoji(text)
	text = utils.RemoveMarkdownSyntax(text) // Remove Markdown syntax
	if text == "" {
		h.logger.Warn("SpeakAndPlay received empty text, cannot synthesize speech, %d, text:%s.", textIndex, originText)
		return errors.New("received empty text, cannot synthesize speech")
	}

	if atomic.LoadInt32(&h.serverVoiceStop) == 1 { // Server voice stopped
		h.LogInfo(fmt.Sprintf("speakAndPlay: server voice stopped, not sending audio data: %s", text))
		text = ""
		return errors.New("server voice stopped, cannot synthesize speech")
	}

	if len(text) > 255 {
		h.logger.Warn(fmt.Sprintf("text exceeds 255 chars limit, truncating speech synthesis: %s", text))
		text = text[:255] // Truncate text
	}

	return nil
}

func (h *ConnectionHandler) clearSpeakStatus() {
	h.LogInfo("[Server] [speaking state] cleared")
	h.tts_last_text_index = -1
	h.providers.asr.Reset() // Reset ASR state
}

func (h *ConnectionHandler) closeOpusDecoder() {
	if h.opusDecoder != nil {
		if err := h.opusDecoder.Close(); err != nil {
			h.LogError(fmt.Sprintf("failed to close Opus decoder: %v", err))
		}
		h.opusDecoder = nil
	}
}

func (h *ConnectionHandler) cleanTTSAndAudioQueue(bClose bool) error {
	msgPrefix := ""
	if bClose {
		msgPrefix = "closing connection: "
	}
	// Abort TTS tasks, clear ttsQueue
	for {
		select {
		case task := <-h.ttsQueue:
			h.LogInfo(fmt.Sprintf(msgPrefix+"discarded TTS task: %s", task.text))
		default:
			// Queue drained, exit loop
			h.LogInfo(msgPrefix + "ttsQueue drained, stopped processing TTS tasks, draining audio queue")
			goto clearAudioQueue
		}
	}

clearAudioQueue:
	// Drain audioMessagesQueue
	for {
		select {
		case task := <-h.audioMessagesQueue:
			h.LogInfo(fmt.Sprintf(msgPrefix+"discarded audio task: %s", task.text))
			// Delete discarded audio files if configured
			h.deleteAudioFileIfNeeded(task.filepath, msgPrefix+"discarding audio task")
		default:
			// Queue drained, exit loop
			h.LogInfo(msgPrefix + "audioMessagesQueue drained, stopped processing audio tasks")
			return nil
		}
	}
}

// Close cleans up resources
func (h *ConnectionHandler) Close() {
	h.closeOnce.Do(func() {
		close(h.stopChan)

		h.closeOpusDecoder()
		if h.providers.tts != nil {
			h.providers.tts.SetVoice(h.initialVoice) // Restore initial voice
		}
		if h.providers.asr != nil {
			h.providers.asr.ResetSilenceCount() // Reset silence count
			if err := h.providers.asr.Reset(); err != nil {
				h.LogError(fmt.Sprintf("failed to reset ASR state: %v", err))
			}
			if err := h.providers.asr.CloseConnection(); err != nil {
				h.LogError(fmt.Sprintf("failed to disconnect ASR state: %v", err))
			}
		}
		h.cleanTTSAndAudioQueue(true)
	})
}

// genResponseByVLLM uses VLLLM to process messages with images
func (h *ConnectionHandler) genResponseByVLLM(ctx context.Context, messages []providers.Message, imageData image.ImageData, text string, round int) error {
	h.logger.Info("generating VLLLM reply %v", map[string]interface{}{
		"text":          text,
		"has_url":       imageData.URL != "",
		"has_data":      imageData.Data != "",
		"format":        imageData.Format,
		"message_count": len(messages),
	})

	// Process image and text using VLLLM
	responses, err := h.providers.vlllm.ResponseWithImage(ctx, h.sessionID, messages, imageData, text)
	if err != nil {
		h.LogError(fmt.Sprintf("VLLLM reply generation failed, falling back to standard LLM: %v", err))
		// Fallback: call standard LLM with text only
		fallbackText := fmt.Sprintf("User sent an image and asked: %s (Note: unable to process image currently, answering based on text only)", text)
		fallbackMessages := append(messages, providers.Message{
			Role:    "user",
			Content: fallbackText,
		})
		return h.genResponseByLLM(ctx, fallbackMessages, round)
	}

	// Process VLLLM streaming response
	var responseMessage []string
	processedChars := 0
	textIndex := 0

	atomic.StoreInt32(&h.serverVoiceStop, 0)

	for response := range responses {
		if response == "" {
			continue
		}

		responseMessage = append(responseMessage, response)
		// Process segmentation
		fullText := utils.JoinStrings(responseMessage)
		currentText := fullText[processedChars:]

		// Split by punctuation
		if segment, chars := utils.SplitAtLastPunctuation(currentText); chars > 0 {
			textIndex++
			h.tts_last_text_index = textIndex
			h.SpeakAndPlay(segment, textIndex, round)
			processedChars += chars
		}
	}

	// Process remaining text
	remainingText := utils.JoinStrings(responseMessage)[processedChars:]
	if remainingText != "" {
		textIndex++
		h.tts_last_text_index = textIndex
		h.SpeakAndPlay(remainingText, textIndex, round)
	}

	// Get complete response content
	content := utils.JoinStrings(responseMessage)

	// Add VLLLM response to dialogue history
	h.dialogueManager.Put(chat.Message{
		Role:    "assistant",
		Content: content,
	})

	h.LogInfo(fmt.Sprintf("VLLLM reply processing completed %v", map[string]interface{}{
		"content_length": len(content),
		"text_segments":  textIndex,
	}))

	return nil
}
