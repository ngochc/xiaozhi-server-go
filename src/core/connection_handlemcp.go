package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/core/types"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/httpsvr/vision"
)

func (h *ConnectionHandler) initMCPResultHandlers() {
	// Initialize MCP result handlers
	// Additional handler initialization logic
	h.mcpResultHandlers = map[string]MCPResultHandler{
		"mcp_handler_exit":         h.mcp_handler_exit,
		"mcp_handler_take_photo":   h.mcp_handler_take_photo,
		"mcp_handler_change_voice": h.mcp_handler_change_voice,
		"mcp_handler_change_role":  h.mcp_handler_change_role,
		"mcp_handler_play_music":   h.mcp_handler_play_music,
		"mcp_handler_switch_agent": h.mcp_handler_switch_agent,
		"mcp_handler_list_agents":  h.mcp_handler_list_agents,
		"mcp_handler_create_agent": h.mcp_handler_create_agent,
		"mcp_handler_update_agent": h.mcp_handler_update_agent,
	}
}

// mcp_handler_switch_agent handles agent switch requests. Parameters can be {"agent_id": <number>} or {"agent_id": "123"} or {"agent_name": "name"}
func (h *ConnectionHandler) mcp_handler_switch_agent(args interface{}) string {
	if !database.IsOnboardingAgent(database.GetDB(), h.agentID) {
		return "Only the onboarding assistant can switch agents"
	}
	var newAgentID uint = 0
	var agentName string

	switch v := args.(type) {
	case map[string]interface{}:
		if idv, ok := v["agent_id"]; ok {
			switch idt := idv.(type) {
			case float64:
				newAgentID = uint(idt)
			case int:
				newAgentID = uint(idt)
			case string:
				if n, err := strconv.Atoi(idt); err == nil {
					newAgentID = uint(n)
				}
			}
		}
		if namev, ok := v["agent_name"]; ok {
			if s, ok2 := namev.(string); ok2 {
				agentName = s
			}
		}
	case string:
		// If string passed directly, attempt numeric ID parse, else treat as name
		if n, err := strconv.Atoi(v); err == nil {
			newAgentID = uint(n)
		} else {
			agentName = v
		}
	case float64:
		newAgentID = uint(v)
	case int:
		newAgentID = uint(v)
	default:
		h.logger.Error("mcp_handler_switch_agent: unsupported arg type %T", v)
		return "Failed to switch agent: invalid parameter format"
	}

	if newAgentID != 0 && newAgentID == h.agentID {
		h.logger.Info("mcp_handler_switch_agent: already using agent %d", newAgentID)
		h.SystemSpeak("You are already using this agent")
		return "You are already using this agent, no need to switch"
	}

	agents, err := database.ListAgentsByUser(database.GetDB(), database.AdminUserID)
	// Find agent
	if err != nil {
		h.logger.Error("mcp_handler_switch_agent: ListAgentsByUser failed: %v", err)
		h.SystemSpeak("Failed to switch agent: unable to get agent list")
		return "Failed to switch agent: unable to get agent list"
	}
	device, err := database.FindDeviceByID(database.GetDB(), h.deviceID) // Ensure device exists
	if err != nil || device == nil {
		h.logger.Error("mcp_handler_switch_agent: FindDeviceByID failed: %v", err)
		h.SystemSpeak("Failed to switch agent: unable to get device information")
		return "Failed to switch agent: unable to get device information"
	}

	for _, ag := range agents {
		if ag.ID == newAgentID || (agentName != "" && ag.Name == agentName) {
			if ag.IsOnboarding {
				return "Already in onboarding assistant, no need to switch"
			}
			// Find matching agent
			h.logger.Info("mcp_handler_switch_agent: found agent %d, name %s", ag.ID, ag.Name)
			h.agentID = ag.ID
			device.AgentID = &ag.ID
			ownerID := ag.UserID
			device.UserID = &ownerID
			device.AuthStatus = "bound"
			if err := database.UpdateDevice(database.GetDB(), device); err != nil {
				h.logger.Error("mcp_handler_switch_agent: UpdateDevice failed: %v", err)
				return "Failed to switch agent: failed to save device binding"
			}
			agent, prompt := h.InitWithAgent()
			// Update dialogue system prompt and retain recent context
			h.dialogueManager.SetSystemMessage(prompt)
			h.dialogueManager.KeepRecentMessages(1)
			// Re-check and switch providers
			h.checkTTSProvider(agent, h.config)
			h.checkLLMProvider(agent, h.config)

			if agent != nil && agent.Name != "" {
				h.SystemSpeak("Switched to " + agent.Name)
			} else {
				h.SystemSpeak("Switched to new agent")
			}
			return "Agent switched successfully"
		}
	}
	h.SystemSpeak("Matching agent not found")
	return "Failed to switch agent: matching agent not found"
}

func (h *ConnectionHandler) handleMCPResultCall(result types.ActionResponse) string {
	errResult := "Tool call failed"
	// Extract result first
	if result.Action != types.ActionTypeCallHandler {
		h.logger.Error("handleMCPResultCall: result.Action is not ActionTypeCallHandler, but %d", result.Action)
		return errResult
	}
	if result.Result == nil {
		h.logger.Error("handleMCPResultCall: result.Result is nil")
		return errResult
	}

	// Extract result.Result struct, including function name and arguments
	if Caller, ok := result.Result.(types.ActionResponseCall); ok {
		if handler, exists := h.mcpResultHandlers[Caller.FuncName]; exists {
			// Call corresponding handler function
			resultStr := handler(Caller.Args)
			return resultStr
		} else {
			h.logger.Error("handleMCPResultCall: no handler found for function %s", Caller.FuncName)
		}
	} else {
		h.logger.Error("handleMCPResultCall: result.Result is not a map[string]interface{}")
	}
	return errResult
}

func (h *ConnectionHandler) mcp_handler_play_music(args interface{}) string {
	if songName, ok := args.(string); ok {
		h.logger.Info("mcp_handler_play_music: %s", songName)
		if path, name, err := utils.GetMusicFilePathFuzzy(songName); err != nil {
			h.logger.Error("mcp_handler_play_music: Play failed: %v", err)
			h.SystemSpeak("Could not find song named " + songName)
			return "Could not find song named " + songName
		} else {
			//h.SystemSpeak("Playing music for you: " + songName)
			h.sendAudioMessage(path, name, h.tts_last_text_index, h.talkRound)
			return "Now playing music: " + name
		}
	} else {
		h.logger.Error("mcp_handler_play_music: args is not a string")
	}
	return "Failed to play music: invalid parameter format"
}

func (h *ConnectionHandler) mcp_handler_change_voice(args interface{}) string {
	if voice, ok := args.(string); ok {
		h.logger.Info("mcp_handler_change_voice: %s", voice)
		if err, voiceName := h.providers.tts.SetVoice(voice); err != nil {
			h.logger.Error("mcp_handler_change_voice: SetVoice failed: %v", err)
			h.SystemSpeak("Failed to switch voice: voice " + voice + " not found")
			return "Failed to switch voice: voice " + voice + " not found"
		} else {
			h.LogInfo(fmt.Sprintf("mcp_handler_change_voice: SetVoice success: %s", voiceName))
			h.SystemSpeak("Switched to voice " + voice)
			return "Voice switched successfully, current voice: " + voiceName
		}
	} else {
		h.logger.Error("mcp_handler_change_voice: args is not a string")
		return "Failed to switch voice: invalid parameter format"
	}
}

func (h *ConnectionHandler) mcp_handler_change_role(args interface{}) string {
	if params, ok := args.(map[string]string); ok {
		role := params["role"]
		prompt := params["prompt"]

		h.logger.Info("mcp_handler_change_role: %s", role)
		h.dialogueManager.SetSystemMessage(prompt)
		h.dialogueManager.KeepRecentMessages(5) // Keep most recent 5 messages
		if getter, ok := h.providers.tts.(ttsConfigGetter); ok {
			ttsProvider := getter.Config().Type
			if ttsProvider == "edge" {
				if role == "Tech Companion" {
					h.providers.tts.SetVoice("zh-CN-shaanxi-XiaoniNeural") // Tech companion voice
				} else if role == "English Tutor" {
					h.providers.tts.SetVoice("zh-CN-XiaoyiNeural") // English tutor voice
				} else if role == "Curious Explorer" {
					h.providers.tts.SetVoice("zh-CN-YunxiNeural") // Curious explorer voice
				}
			}
		}
		h.SystemSpeak("Switched to new role: " + role)
		return "Role switched successfully: " + role
	} else {
		h.logger.Error("mcp_handler_change_role: args is not a string")
		return "Failed to switch role: invalid parameter format"
	}
}

func (h *ConnectionHandler) mcp_handler_exit(args interface{}) string {
	if text, ok := args.(string); ok {
		h.closeAfterChat = true
		h.SystemSpeak(text)
		return "Ending conversation: " + text
	} else {
		h.logger.Error("mcp_handler_exit: args is not a string")
		return "Failed to end conversation: invalid parameter format"
	}
}

func (h *ConnectionHandler) mcp_handler_take_photo(args interface{}) string {
	// Handle photo capture function, parse into VisionResponse
	resultStr, _ := args.(string)
	var visionResponse vision.VisionResponse
	if err := json.Unmarshal([]byte(resultStr), &visionResponse); err != nil {
		h.logger.Error("failed to parse VisionResponse: %v", err)
		return "Photo capture failed: unable to parse response"
	}

	if !visionResponse.Success {
		h.logger.Error("photo capture failed: %s", visionResponse.Message)
		h.genResponseByLLM(context.Background(), h.dialogueManager.GetLLMDialogue(), h.talkRound)
		return "Photo capture failed: " + visionResponse.Message
	}

	h.SystemSpeak(visionResponse.Result)
	return "Photo capture successful: " + visionResponse.Result
}
