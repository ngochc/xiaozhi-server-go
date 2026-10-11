package webapi

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"xiaozhi-server-go/src/configs"

	"github.com/gin-gonic/gin"
)

const diagnosticTimeout = 3 * time.Second

type DiagnosticResult struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latencyMs,omitempty"`
}

func (s *SystemConfigService) handleRunDiagnostics(c *gin.Context) {
	c.JSON(200, gin.H{
		"status": "ok",
		"data":   runDiagnostics(configs.Cfg),
	})
}

func runDiagnostics(config *configs.Config) []DiagnosticResult {
	if config == nil {
		return []DiagnosticResult{{Name: "Server Configuration", Status: "error", Message: "Server configuration not yet initialized"}}
	}

	results := []DiagnosticResult{
		diagnoseWebSocket(config.Web.Websocket),
		diagnoseLocalHTTP(config.Web.Port),
		diagnoseASR(config),
		diagnoseLLM(config),
		diagnoseTTS(config),
	}
	return results
}

func diagnoseWebSocket(address string) DiagnosticResult {
	if isPlaceholder(address) {
		return invalidConfigResult("WebSocket", "Please enter WebSocket URL accessible by devices")
	}
	parsed, err := url.Parse(address)
	if err != nil || (parsed.Scheme != "ws" && parsed.Scheme != "wss") || parsed.Host == "" {
		return invalidConfigResult("WebSocket", "URL must start with ws:// or wss://")
	}
	return probeEndpoint("WebSocket", address)
}

func diagnoseLocalHTTP(port int) DiagnosticResult {
	if port <= 0 || port > 65535 {
		return invalidConfigResult("OTA / Web Admin", "Invalid Web port")
	}
	return probeEndpoint("OTA / Web Admin", fmt.Sprintf("http://127.0.0.1:%d", port))
}

func diagnoseASR(config *configs.Config) DiagnosticResult {
	name := config.SelectedModule["ASR"]
	provider, ok := config.ASR[name]
	if name == "" || !ok {
		return invalidConfigResult("ASR", "No valid ASR provider selected")
	}
	if hasPlaceholderMap(provider, "appid", "access_token", "api_key", "api_secret") {
		return invalidConfigResult("ASR", "Credentials not filled for selected ASR provider")
	}
	if address := firstString(provider, "addr", "asr_url", "url", "cluster"); address != "" {
		return probeEndpoint("ASR", address)
	}
	return DiagnosticResult{Name: "ASR", Status: "ready", Message: "Config filled; provider has no standalone endpoint to probe"}
}

func diagnoseLLM(config *configs.Config) DiagnosticResult {
	name := config.SelectedModule["LLM"]
	provider, ok := config.LLM[name]
	if name == "" || !ok {
		return invalidConfigResult("LLM", "No valid LLM provider selected")
	}
	if provider.Type != "coze" && isPlaceholder(provider.ModelName) {
		return invalidConfigResult("LLM", "Selected LLM provider is missing model name")
	}
	if provider.Type == "coze" {
		if !hasUsableCozeCredential(provider.Extra) {
			return invalidConfigResult("LLM", "Selected Coze provider is missing credentials")
		}
	} else if provider.Type != "ollama" && isPlaceholder(provider.APIKey) {
		return invalidConfigResult("LLM", "Selected LLM provider is missing API key")
	}
	if isPlaceholder(provider.BaseURL) {
		return invalidConfigResult("LLM", "Selected LLM provider is missing service URL")
	}
	return probeEndpoint("LLM", provider.BaseURL)
}

func diagnoseTTS(config *configs.Config) DiagnosticResult {
	name := config.SelectedModule["TTS"]
	provider, ok := config.TTS[name]
	if name == "" || !ok {
		return invalidConfigResult("TTS", "No valid TTS provider selected")
	}
	switch provider.Type {
	case "edge":
		return DiagnosticResult{Name: "TTS", Status: "ready", Message: "Edge TTS configured; service connection will be established on first speech output"}
	case "doubao", "iflytek":
		if hasPlaceholderStrings(provider.AppID, provider.Token, provider.Cluster) {
			return invalidConfigResult("TTS", "Selected TTS provider is missing credentials")
		}
	case "deepgram":
		if hasPlaceholderStrings(provider.Token, provider.Cluster) {
			return invalidConfigResult("TTS", "Selected TTS provider is missing credentials")
		}
	case "gosherpa":
		if isPlaceholder(provider.Cluster) {
			return invalidConfigResult("TTS", "Selected TTS provider is missing service URL")
		}
	}
	if provider.Cluster != "" && !isPlaceholder(provider.Cluster) && looksLikeEndpoint(provider.Cluster) {
		return probeEndpoint("TTS", provider.Cluster)
	}
	return DiagnosticResult{Name: "TTS", Status: "ready", Message: "Config filled; provider has no standalone endpoint to probe"}
}

func probeEndpoint(name, address string) DiagnosticResult {
	host, err := endpointHost(address)
	if err != nil {
		return invalidConfigResult(name, "Invalid service URL format")
	}
	started := time.Now()
	connection, err := net.DialTimeout("tcp", host, diagnosticTimeout)
	latency := time.Since(started).Milliseconds()
	if err != nil {
		return DiagnosticResult{Name: name, Status: "error", Message: "Unable to connect to service URL", LatencyMS: latency}
	}
	_ = connection.Close()
	return DiagnosticResult{Name: name, Status: "ok", Message: "Address reachable", LatencyMS: latency}
}

func endpointHost(address string) (string, error) {
	if !strings.Contains(address, "://") {
		address = "tcp://" + address
	}
	parsed, err := url.Parse(address)
	if err != nil || parsed.Hostname() == "" {
		return "", fmt.Errorf("invalid endpoint")
	}
	port := parsed.Port()
	if port == "" {
		switch parsed.Scheme {
		case "https", "wss":
			port = "443"
		default:
			port = "80"
		}
	}
	return net.JoinHostPort(parsed.Hostname(), port), nil
}

func firstString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func hasPlaceholderMap(values map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		if value, exists := values[key]; exists {
			if text, ok := value.(string); !ok || isPlaceholder(text) {
				return true
			}
		}
	}
	return false
}

func hasPlaceholderStrings(values ...string) bool {
	for _, value := range values {
		if isPlaceholder(value) {
			return true
		}
	}
	return false
}

func hasUsableCozeCredential(values map[string]interface{}) bool {
	for _, key := range []string{"personal_access_token", "client_id"} {
		if value, ok := values[key].(string); ok && !isPlaceholder(value) {
			return true
		}
	}
	return false
}

func isPlaceholder(value string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	return trimmed == "" || strings.Contains(trimmed, "your_") || strings.Contains(trimmed, "your ")
}

func looksLikeEndpoint(value string) bool {
	return strings.Contains(value, "://") || strings.Contains(value, ":")
}

func invalidConfigResult(name, message string) DiagnosticResult {
	return DiagnosticResult{Name: name, Status: "error", Message: message}
}
