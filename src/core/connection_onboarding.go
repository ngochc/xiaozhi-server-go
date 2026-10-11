package core

import (
	"fmt"
	"strings"

	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/models"
)

func (h *ConnectionHandler) requireOnboardingAgent() bool {
	if database.IsOnboardingAgent(database.GetDB(), h.agentID) {
		return true
	}
	h.LogInfo("Rejecting non-onboarding assistant call to agent management tool")
	return false
}

func stringArg(args interface{}, key string) string {
	if values, ok := args.(map[string]interface{}); ok {
		if value, ok := values[key].(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (h *ConnectionHandler) mcp_handler_list_agents(args interface{}) string {
	if !h.requireOnboardingAgent() {
		return "Only the onboarding assistant can view switchable agents"
	}
	agents, err := database.ListAgentsByUser(database.GetDB(), database.AdminUserID)
	if err != nil {
		h.logger.Error("failed to get agent list: %v", err)
		return "Failed to get agent list"
	}
	items := make([]string, 0, len(agents))
	for _, agent := range agents {
		if agent.IsOnboarding {
			continue
		}
		description := strings.TrimSpace(agent.Description)
		if description == "" {
			description = "No description configured"
		}
		items = append(items, fmt.Sprintf("ID %d: %s (%s, LLM: %s, Voice: %s)", agent.ID, agent.Name, description, agent.LLM, agent.Voice))
	}
	if len(items) == 0 {
		return "No other preset agents available yet. You can create one per user request."
	}
	return "Switchable preset agents: " + strings.Join(items, "; ")
}

func (h *ConnectionHandler) mcp_handler_create_agent(args interface{}) string {
	if !h.requireOnboardingAgent() {
		return "Only the onboarding assistant can create agents"
	}
	name := stringArg(args, "name")
	if name == "" {
		return "Failed to create agent: please confirm the new agent name first"
	}
	llmName := stringArg(args, "llm")
	if llmName == "" {
		llmName = h.config.SelectedModule["LLM"]
	}
	if _, ok := h.config.LLM[llmName]; !ok {
		return fmt.Sprintf("Failed to create agent: configured LLM %q not found", llmName)
	}
	voice := stringArg(args, "voice")
	if voice == "" {
		if ttsName := h.config.SelectedModule["TTS"]; ttsName != "" {
			voice = h.config.TTS[ttsName].Voice
		}
	}
	agent := &models.Agent{
		Name:        name,
		LLM:         llmName,
		Language:    "English",
		Voice:       voice,
		Prompt:      stringArg(args, "prompt"),
		Description: stringArg(args, "description"),
		UserID:      database.AdminUserID,
	}
	if err := database.CreateAgent(database.GetDB(), agent); err != nil {
		h.logger.Error("failed to create agent: %v", err)
		return "Failed to create agent"
	}
	return fmt.Sprintf("Created agent %q (ID %d), LLM: %s, Voice: %s", agent.Name, agent.ID, agent.LLM, agent.Voice)
}

func (h *ConnectionHandler) mcp_handler_update_agent(args interface{}) string {
	if !h.requireOnboardingAgent() {
		return "Only the onboarding assistant can modify agents"
	}
	targetName := stringArg(args, "agent_name")
	if targetName == "" {
		return "Failed to modify agent: please confirm the agent name to modify first"
	}
	agents, err := database.ListAgentsByUser(database.GetDB(), database.AdminUserID)
	if err != nil {
		h.logger.Error("failed to get agent list: %v", err)
		return "Failed to modify agent: unable to get agent list"
	}
	var target *models.Agent
	for index := range agents {
		if agents[index].Name == targetName {
			target = &agents[index]
			break
		}
	}
	if target == nil {
		return fmt.Sprintf("Failed to modify agent: %q not found", targetName)
	}
	if target.IsOnboarding {
		return "The onboarding assistant is system-managed and cannot be modified via chat"
	}

	changes := make([]string, 0, 4)
	if value := stringArg(args, "new_name"); value != "" && value != target.Name {
		target.Name = value
		changes = append(changes, "name")
	}
	if value := stringArg(args, "llm"); value != "" && value != target.LLM {
		if _, ok := h.config.LLM[value]; !ok {
			return fmt.Sprintf("Failed to modify agent: configured LLM %q not found", value)
		}
		target.LLM = value
		changes = append(changes, "LLM")
	}
	if value := stringArg(args, "voice"); value != "" && value != target.Voice {
		target.Voice = value
		changes = append(changes, "voice")
	}
	if value := stringArg(args, "prompt"); value != "" && value != target.Prompt {
		target.Prompt = value
		changes = append(changes, "prompt")
	}
	if value := stringArg(args, "description"); value != "" && value != target.Description {
		target.Description = value
		changes = append(changes, "description")
	}
	if len(changes) == 0 {
		return "No fields received for modification"
	}
	if err := database.UpdateAgent(database.GetDB(), target); err != nil {
		h.logger.Error("failed to update agent: %v", err)
		return "Failed to modify agent"
	}
	return fmt.Sprintf("Modified agent %q: %s", target.Name, strings.Join(changes, ", "))
}
