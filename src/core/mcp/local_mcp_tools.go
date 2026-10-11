package mcp

import (
	"context"
	"strings"
	"time"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/types"
)

func (c *LocalClient) AddToolExit() error {
	InputSchema := ToolInputSchema{
		Type: "object",
		Properties: map[string]any{
			"say_goodbye": map[string]any{
				"type":        "string",
				"description": "Friendly farewell phrase to end the conversation",
			},
		},
		Required: []string{"say_goodbye"},
	}

	c.AddTool("exit",
		"Call when the user wants to end the conversation or exit the system",
		InputSchema,
		func(ctx context.Context, args map[string]any) (interface{}, error) {
			c.logger.Info("User requested exit, farewell: %s", args["say_goodbye"])
			res := types.ActionResponse{
				Action: types.ActionTypeCallHandler, // Action type
				Result: types.ActionResponseCall{
					FuncName: "mcp_handler_exit",  // Function name
					Args:     args["say_goodbye"], // Function argument
				},
			}
			return res, nil
		})

	return nil
}

func (c *LocalClient) AddToolTime() error {
	InputSchema := ToolInputSchema{
		Type:       "object",
		Properties: map[string]any{},
		Required:   []string{},
	}

	c.AddTool("get_time",
		"Call when the user asks for today's date or the current time",
		InputSchema,
		func(ctx context.Context, args map[string]any) (interface{}, error) {
			now := time.Now()
			time := now.Format("2006-01-02 15:04:05")
			week := now.Weekday().String()
			str := "Current time is " + time + ", today is " + week + "."
			res := types.ActionResponse{
				Action: types.ActionTypeReqLLM, // Action type
				Result: str,                    // Function result
			}
			return res, nil
		})

	return nil
}

func (c *LocalClient) AddToolChangeRole() error {
	roles := c.cfg.Roles
	prompts := map[string]string{}
	roleNames := ""
	if roles == nil {
		c.logger.Warn(
			"AddToolChangeRole: roles settings is nil or empty, Skipping tool registration",
		)
		return nil
	} else {
		for _, role := range roles {

			prompts[role.Name] = role.Description
			roleNames += role.Name + ", "
		}
	}

	InputSchema := ToolInputSchema{
		Type: "object",
		Properties: map[string]any{
			"role": map[string]any{
				"type":        "string",
				"description": "New persona/role name",
			},
		},
		Required: []string{"role"},
	}

	c.AddTool("change_role",
		"Call when user wants to switch persona/role/assistant name. Available roles: ["+roleNames+"]",
		InputSchema,
		func(ctx context.Context, args map[string]any) (interface{}, error) {
			role := args["role"].(string)
			res := types.ActionResponse{
				Action: types.ActionTypeCallHandler, // Action type
				Result: types.ActionResponseCall{
					FuncName: "mcp_handler_change_role", // Function name
					Args: map[string]string{
						"role":   role, // Function parameter
						"prompt": prompts[role],
					},
				},
			}
			return res, nil
		})

	return nil
}

func (c *LocalClient) AddToolChangeVoice() error {
	voices := []configs.VoiceInfo{}
	if ttsType, ok := c.cfg.SelectedModule["TTS"]; ok && ttsType != "" {
		voices = c.cfg.TTS[ttsType].SupportedVoices
	}
	voiceDesArr := []string{}
	for _, v := range voices {
		voiceDesArr = append(voiceDesArr, v.Name+"("+v.DisplayName+"-"+v.Sex+")："+v.Description)
	}
	voiceDes := strings.Join(voiceDesArr, ", ")

	InputSchema := ToolInputSchema{
		Type: "object",
		Properties: map[string]any{
			"voice": map[string]any{
				"type":        "string",
				"description": "New voice name, the first part of the voice description",
			},
		},
		Required: []string{"voice"},
	}

	c.AddTool("change_voice",
		"Call when user wants to change persona voice. Supported voices: "+voiceDes,
		InputSchema,
		func(ctx context.Context, args map[string]any) (interface{}, error) {
			voice := args["voice"].(string)
			res := types.ActionResponse{
				Action: types.ActionTypeCallHandler, // Action type
				Result: types.ActionResponseCall{
					FuncName: "mcp_handler_change_voice", // Function name
					Args:     voice,                      // Function argument
				},
			}
			return res, nil
		})

	return nil
}

func (c *LocalClient) AddToolPlayMusic() error {
	InputSchema := ToolInputSchema{
		Type: "object",
		Properties: map[string]any{
			"song_name": map[string]any{
				"type":        "string",
				"description": "Song name, or 'random' if user does not specify a specific song. Example: User: play music -> Parameter: random",
			},
		},
		Required: []string{"song_name"},
	}

	c.AddTool("play_music",
		"Call when the user wants to play music or listen to songs",
		InputSchema,
		func(ctx context.Context, args map[string]any) (interface{}, error) {
			song_name := args["song_name"].(string)
			res := types.ActionResponse{
				Action: types.ActionTypeCallHandler, // Action type
				Result: types.ActionResponseCall{
					FuncName: "mcp_handler_play_music", // Function name
					Args:     song_name,                // Function argument
				},
			}
			return res, nil
		})

	return nil
}

func (c *LocalClient) AddToolSwitchAgent() error {
	InputSchema := ToolInputSchema{
		Type: "object",
		Properties: map[string]any{
			"agent_id": map[string]any{
				"type":        "number",
				"description": "Target agent ID to switch to",
			},
			"agent_name": map[string]any{
				"type":        "string",
				"description": "Agent name",
			},
		},
		Required: []string{},
	}

	c.AddTool("switch_agent",
		"Used by setup assistant: call when user wants to switch to agent by name or ID. Must provide agent_id or agent_name.",
		InputSchema,
		func(ctx context.Context, args map[string]any) (interface{}, error) {
			// Verify at least one parameter is provided
			if _, hasID := args["agent_id"]; !hasID {
				if _, hasName := args["agent_name"]; !hasName {
					c.logger.Warn("switch_agent: must provide either agent_id or agent_name")
					return types.ActionResponse{
						Action: types.ActionTypeReqLLM,
						Result: "Agent ID or name is required to switch agent",
					}, nil
				}
			}

			// Pass parameters to handler to execute switch
			res := types.ActionResponse{
				Action: types.ActionTypeCallHandler,
				Result: types.ActionResponseCall{
					FuncName: "mcp_handler_switch_agent",
					Args:     args,
				},
			}
			return res, nil
		})

	return nil
}

func (c *LocalClient) AddToolListAgents() error {
	inputSchema := ToolInputSchema{Type: "object", Properties: map[string]any{}, Required: []string{}}
	return c.AddTool("list_agents",
		"Used by setup assistant: call when user asks what agents are available to switch to.",
		inputSchema,
		func(ctx context.Context, args map[string]any) (interface{}, error) {
			return types.ActionResponse{Action: types.ActionTypeCallHandler, Result: types.ActionResponseCall{
				FuncName: "mcp_handler_list_agents", Args: args,
			}}, nil
		})
}

func (c *LocalClient) AddToolCreateAgent() error {
	inputSchema := ToolInputSchema{
		Type: "object",
		Properties: map[string]any{
			"name":        map[string]any{"type": "string", "description": "New agent name"},
			"llm":         map[string]any{"type": "string", "description": "Configured LLM provider name; uses default if omitted"},
			"voice":       map[string]any{"type": "string", "description": "TTS voice ID; uses default if omitted"},
			"prompt":      map[string]any{"type": "string", "description": "Agent system prompt"},
			"description": map[string]any{"type": "string", "description": "Short description of agent purpose"},
		},
		Required: []string{"name"},
	}
	return c.AddTool("create_agent",
		"Used by setup assistant: call when user requests creating a new agent and provides a name.",
		inputSchema,
		func(ctx context.Context, args map[string]any) (interface{}, error) {
			return types.ActionResponse{Action: types.ActionTypeCallHandler, Result: types.ActionResponseCall{
				FuncName: "mcp_handler_create_agent", Args: args,
			}}, nil
		})
}

func (c *LocalClient) AddToolUpdateAgent() error {
	inputSchema := ToolInputSchema{
		Type: "object",
		Properties: map[string]any{
			"agent_name":  map[string]any{"type": "string", "description": "Target agent name to modify"},
			"new_name":    map[string]any{"type": "string", "description": "New name for agent"},
			"llm":         map[string]any{"type": "string", "description": "Configured LLM provider name"},
			"voice":       map[string]any{"type": "string", "description": "TTS voice ID"},
			"prompt":      map[string]any{"type": "string", "description": "Replacement persona prompt"},
			"description": map[string]any{"type": "string", "description": "Replacement description"},
		},
		Required: []string{"agent_name"},
	}
	return c.AddTool("update_agent",
		"Used by setup assistant: call when user requests modifying an agent name, LLM, voice, or prompt.",
		inputSchema,
		func(ctx context.Context, args map[string]any) (interface{}, error) {
			return types.ActionResponse{Action: types.ActionTypeCallHandler, Result: types.ActionResponseCall{
				FuncName: "mcp_handler_update_agent", Args: args,
			}}, nil
		})
}
