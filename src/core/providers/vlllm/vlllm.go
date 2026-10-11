package vlllm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/image"
	"xiaozhi-server-go/src/core/providers"
	"xiaozhi-server-go/src/core/utils"

	"github.com/sashabaranov/go-openai"
)

// Config VLLLM configuration struct
type Config struct {
	Type        string
	ModelName   string
	BaseURL     string
	APIKey      string
	Temperature float64
	MaxTokens   int
	TopP        float64
	Security    configs.SecurityConfig
	Data        map[string]interface{}
}

// Provider VLLLM provider handling multimodal APIs directly
type Provider struct {
	config         *Config
	imageProcessor *image.ImageProcessor
	logger         *utils.Logger

	// Direct API clients
	openaiClient *openai.Client // For OpenAI type
	httpClient   *http.Client   // For Ollama type
}

// OllamaRequest Ollama API request struct
type OllamaRequest struct {
	Model    string                 `json:"model"`
	Messages []OllamaMessage        `json:"messages"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

// OllamaMessage Ollama message struct
type OllamaMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"` // Base64 encoded images
}

// OllamaResponse Ollama API response struct
type OllamaResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Message   struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// NewProvider creates new VLLLM provider
func NewProvider(config *Config, logger *utils.Logger) (*Provider, error) {
	// Build VLLLM configuration
	vlllmConfig := &configs.VLLMConfig{
		Type:        config.Type,
		ModelName:   config.ModelName,
		BaseURL:     config.BaseURL,
		APIKey:      config.APIKey,
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
		TopP:        config.TopP,
		Security:    config.Security,
	}

	// Create image processor
	imageProcessor, err := image.NewImageProcessor(vlllmConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create image processor: %v", err)
	}

	provider := &Provider{
		config:         config,
		imageProcessor: imageProcessor,
		logger:         logger,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}

	return provider, nil
}

// Initialize initializes provider
func (p *Provider) Initialize() error {
	// Initialize client based on type
	switch strings.ToLower(p.config.Type) {
	case "openai":
		if p.config.APIKey == "" {
			return fmt.Errorf("OpenAI API key is required")
		}

		clientConfig := openai.DefaultConfig(p.config.APIKey)
		if p.config.BaseURL != "" {
			clientConfig.BaseURL = p.config.BaseURL
		}
		p.openaiClient = openai.NewClientWithConfig(clientConfig)

	case "ollama":
		// Ollama requires BaseURL but no API key
		if p.config.BaseURL == "" {
			p.config.BaseURL = "http://localhost:11434" // Default Ollama URL
		}
		p.logger.Debug("Ollama VLLLM initialized successfully %v", map[string]interface{}{
			"base_url": p.config.BaseURL,
			"model":    p.config.ModelName,
		})

	default:
		return fmt.Errorf("unsupported VLLLM type: %s", p.config.Type)
	}

	p.logger.Debug("VLLLM provider initialized successfully %v", map[string]interface{}{
		"type":       p.config.Type,
		"model_name": p.config.ModelName,
	})

	return nil
}

// Cleanup cleans up resources
func (p *Provider) Cleanup() error {
	// Clean up image processor
	if err := p.imageProcessor.Cleanup(); err != nil {
		p.logger.Warn("failed to cleanup image processor", err)
	}

	p.logger.Info("VLLLM provider cleanup completed")
	return nil
}

// ResponseWithImage handles requests containing images
func (p *Provider) ResponseWithImage(ctx context.Context, sessionID string, messages []providers.Message, imageData image.ImageData, text string) (<-chan string, error) {
	// Process image
	base64Image, err := p.imageProcessor.ProcessImage(ctx, imageData)
	if err != nil {
		return nil, fmt.Errorf("image processing failed: %v", err)
	}

	p.logger.Debug("calling multimodal API %v", map[string]interface{}{
		"type":       p.config.Type,
		"model_name": p.config.ModelName,
		"text":       text,
		"image_size": len(base64Image),
	})

	// Call corresponding multimodal API based on type
	switch strings.ToLower(p.config.Type) {
	case "openai":
		return p.responseWithOpenAIVision(ctx, messages, base64Image, text, imageData.Format)
	case "ollama":
		return p.responseWithOllamaVision(ctx, messages, base64Image, text, imageData.Format)
	default:
		return nil, fmt.Errorf("unsupported VLLLM type: %s", p.config.Type)
	}
}

// responseWithOpenAIVision invokes OpenAI Vision API
func (p *Provider) responseWithOpenAIVision(ctx context.Context, messages []providers.Message, base64Image string, text string, format string) (<-chan string, error) {
	responseChan := make(chan string, 10)

	go func() {
		defer close(responseChan)

		// Construct OpenAI multimodal message
		chatMessages := make([]openai.ChatCompletionMessage, 0, len(messages)+1)

		// Add conversation history
		for _, msg := range messages {
			chatMessages = append(chatMessages, openai.ChatCompletionMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}

		// Construct multimodal message with image
		visionMessage := openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleUser,
			MultiContent: []openai.ChatMessagePart{
				{
					Type: openai.ChatMessagePartTypeText,
					Text: text,
				},
				{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL: fmt.Sprintf("data:image/%s;base64,%s", format, base64Image),
					},
				},
			},
		}
		// Log visionMessage content
		p.logger.Debug("constructed OpenAI Vision message: %v", visionMessage)
		chatMessages = append(chatMessages, visionMessage)

		// Call OpenAI Vision API
		stream, err := p.openaiClient.CreateChatCompletionStream(
			ctx,
			openai.ChatCompletionRequest{
				Model:       p.config.ModelName,
				Messages:    chatMessages,
				Stream:      true,
				Temperature: float32(p.config.Temperature),
				TopP:        float32(p.config.TopP),
			},
		)
		if err != nil {
			responseChan <- fmt.Sprintf("【VLLLM service response error: %v】", err)
			p.logger.Error("OpenAI Vision API call failed %v", err)
			p.logger.Info("OpenAI Vision API call failed, %s, maxTokens:%dm, Temperature:%f, top:%f", p.config.ModelName, p.config.MaxTokens, float32(p.config.Temperature), float32(p.config.TopP))

			return
		}
		defer stream.Close()

		p.logger.Info("OpenAI Vision API call succeeded, receiving streaming response")

		isActive := true
		for {
			response, err := stream.Recv()
			if err != nil {
				break
			}

			if len(response.Choices) > 0 {
				content := response.Choices[0].Delta.Content
				if content != "" {
					// Process think tags
					if content, isActive = p.handleThinkTags(content, isActive); content != "" {
						responseChan <- content
					}
				}
			}
		}

		p.logger.Info("OpenAI Vision API streaming response completed")
	}()

	return responseChan, nil
}

// responseWithOllamaVision invokes Ollama Vision API
func (p *Provider) responseWithOllamaVision(ctx context.Context, messages []providers.Message, base64Image string, text string, format string) (<-chan string, error) {
	responseChan := make(chan string, 10)

	go func() {
		defer close(responseChan)

		// Construct Ollama request
		ollamaMessages := make([]OllamaMessage, 0, len(messages)+1)

		// Add conversation history
		for _, msg := range messages {
			ollamaMessages = append(ollamaMessages, OllamaMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}

		// Add user message with image
		visionMessage := OllamaMessage{
			Role:    "user",
			Content: text,
			Images:  []string{base64Image}, // Ollama requires raw base64 without data URL prefix
		}
		ollamaMessages = append(ollamaMessages, visionMessage)

		// Build request
		request := OllamaRequest{
			Model:    p.config.ModelName,
			Messages: ollamaMessages,
			Stream:   true,
			Options: map[string]interface{}{
				"temperature": p.config.Temperature,
				"top_p":       p.config.TopP,
			},
		}

		// Serialize request
		requestBody, err := json.Marshal(request)
		if err != nil {
			responseChan <- fmt.Sprintf("【Request serialization failed: %v】", err)
			p.logger.Error("Ollama request serialization failed", err)
			return
		}

		// Send request to Ollama
		url := fmt.Sprintf("%s/api/chat", strings.TrimSuffix(p.config.BaseURL, "/"))
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
		if err != nil {
			responseChan <- fmt.Sprintf("【Failed to create request: %v】", err)
			p.logger.Error("failed to create Ollama request", err)
			return
		}

		req.Header.Set("Content-Type", "application/json")

		p.logger.Info("sending multimodal request to Ollama", map[string]interface{}{
			"url":   url,
			"model": p.config.ModelName,
			"text":  text,
		})

		resp, err := p.httpClient.Do(req)
		if err != nil {
			responseChan <- fmt.Sprintf("【Ollama API call failed: %v】", err)
			p.logger.Error("Ollama API call failed", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			responseChan <- fmt.Sprintf("【Ollama API error: %d】", resp.StatusCode)
			p.logger.Error("Ollama API returned error", map[string]interface{}{
				"status_code": resp.StatusCode,
				"status":      resp.Status,
			})
			return
		}

		p.logger.Info("Ollama Vision API call succeeded, receiving streaming response")

		// Process streaming response
		decoder := json.NewDecoder(resp.Body)
		isActive := true

		for {
			var response OllamaResponse
			if err := decoder.Decode(&response); err != nil {
				if err.Error() != "EOF" {
					p.logger.Error("failed to parse Ollama response", err)
				}
				break
			}

			content := response.Message.Content
			if content != "" {
				// Process think tags
				if content, isActive = p.handleThinkTags(content, isActive); content != "" {
					responseChan <- content
				}
			}

			if response.Done {
				break
			}
		}

		p.logger.Info("Ollama Vision API streaming response completed")
	}()

	return responseChan, nil
}

// Response plain text fallback
func (p *Provider) Response(ctx context.Context, sessionID string, messages []providers.Message) (<-chan string, error) {
	// If no image, handle as plain text
	responseChan := make(chan string, 1)
	go func() {
		defer close(responseChan)
		responseChan <- "VLLLM Provider only supports image processing, use LLM Provider for plain text"
	}()
	return responseChan, nil
}

// handleThinkTags processes thinking tags
func (p *Provider) handleThinkTags(content string, isActive bool) (string, bool) {
	if content == "" {
		return "", isActive
	}

	if content == "<think>" {
		return "", false
	}
	if content == "</think>" {
		return "", true
	}

	if !isActive {
		return "", isActive
	}

	return content, isActive
}

// detectMultimodalMessage detects multimodal message format
func (p *Provider) detectMultimodalMessage(content string) (text string, imageURL string, detected bool) {
	// Regex matches previous multimodal message format
	multimodalPattern := regexp.MustCompile(`\[MULTIMODAL_MESSAGE\](.*?)\[/MULTIMODAL_MESSAGE\]`)
	matches := multimodalPattern.FindStringSubmatch(content)

	if len(matches) > 0 {
		// Legacy format multimodal message needs parsing
		// ResponseWithImage should be used directly
		return "", "", true
	}

	return content, "", false
}

// GetImageMetrics gets image processing metrics
func (p *Provider) GetImageMetrics() image.ImageMetrics {
	return p.imageProcessor.GetMetrics()
}

// GetConfig gets configuration info
func (p *Provider) GetConfig() *Config {
	return p.config
}
