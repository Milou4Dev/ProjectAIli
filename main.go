package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

const (
	maxTokens              = 8000
	apiURL                 = "https://api.groq.com/openai/v1/chat/completions"
	initialHistoryCapacity = 10
	configFile             = "config.yaml"
	timeoutSeconds         = 30
	exitCommand            = "exit"
	maxRetries             = 3
	backoffFactor          = 2
	initialBackoff         = 1 * time.Second
	maxConversationTokens  = 4000
	systemPromptFile       = "system_prompt.txt"
	requestsPerSecond      = 10
)

type Config struct {
	GroqAPIKey string `yaml:"groq_api_key"`
}

type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"-"`
}

type APIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Conversation struct {
	History    []Message
	mu         sync.RWMutex
	tokenCount int
}

type APIClient struct {
	httpClient  *http.Client
	apiKey      string
	rateLimiter *time.Ticker
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("%sError: %v%s\n", colorRed, err, colorReset)
	}
}

func run() error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	apiClient := newAPIClient(config.GroqAPIKey)
	conversation, err := newConversation()
	if err != nil {
		return fmt.Errorf("failed to create conversation: %w", err)
	}

	printWelcomeMessage()
	return runChatLoop(apiClient, conversation)
}

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.GroqAPIKey == "" {
		return nil, errors.New("GroqAPIKey is missing in the config file")
	}

	return &config, nil
}

func newAPIClient(apiKey string) *APIClient {
	return &APIClient{
		httpClient: &http.Client{
			Timeout: time.Second * timeoutSeconds,
			Transport: &http.Transport{
				TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
				MaxIdleConns:        100,
				MaxConnsPerHost:     100,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  true,
				ForceAttemptHTTP2:   true,
				MaxIdleConnsPerHost: 100,
			},
		},
		apiKey:      apiKey,
		rateLimiter: time.NewTicker(time.Second / requestsPerSecond),
	}
}

func newConversation() (*Conversation, error) {
	systemPrompt, err := loadSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to load system prompt: %w", err)
	}

	return &Conversation{
		History:    []Message{{Role: "system", Content: systemPrompt, Timestamp: time.Now()}},
		tokenCount: len(strings.Fields(systemPrompt)),
	}, nil
}

func loadSystemPrompt() (string, error) {
	data, err := os.ReadFile(systemPromptFile)
	if err != nil {
		return "", fmt.Errorf("failed to read system prompt file: %w", err)
	}
	return string(data), nil
}

func printWelcomeMessage() {
	clearScreen()
	width, _, _ := term.GetSize(int(os.Stdout.Fd()))
	welcomeMsg := "Welcome to the AI Chat!"
	border := strings.Repeat("─", width-4)

	fmt.Printf("%s┌%s┐\n", colorCyan, border)
	fmt.Printf("│%s%s%s│\n", strings.Repeat(" ", (width-len(welcomeMsg)-2)/2), welcomeMsg, strings.Repeat(" ", (width-len(welcomeMsg)-1)/2))
	fmt.Printf("└%s┘%s\n", border, colorReset)
	fmt.Printf("%sType '%s' to exit the program.%s\n\n", colorBlue, exitCommand, colorReset)
}

func runChatLoop(apiClient *APIClient, conversation *Conversation) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return handleInterrupt(ctx)
	})

	g.Go(func() error {
		return processChatInputLoop(ctx, apiClient, conversation)
	})

	return g.Wait()
}

func handleInterrupt(ctx context.Context) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
		fmt.Printf("\n%sReceived interrupt signal. Exiting...%s\n", colorYellow, colorReset)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func processChatInputLoop(ctx context.Context, apiClient *APIClient, conversation *Conversation) error {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := processChatInput(ctx, scanner, apiClient, conversation); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
		}
	}
}

func processChatInput(ctx context.Context, scanner *bufio.Scanner, apiClient *APIClient, conversation *Conversation) error {
	userInput := getUserInput(scanner)
	if userInput == "" {
		return nil
	}
	if strings.EqualFold(userInput, exitCommand) {
		fmt.Printf("%sGoodbye!%s\n", colorYellow, colorReset)
		return io.EOF
	}

	if strings.HasPrefix(userInput, "/save") {
		return handleSaveCommand(conversation)
	}

	if strings.HasPrefix(userInput, "/load") {
		return handleLoadCommand(userInput, conversation)
	}

	conversation.addMessage("user", userInput)

	aiResponse, err := getAIResponseWithRetry(ctx, apiClient, conversation)
	if err != nil {
		fmt.Printf("%sFailed to get AI response: %v%s\n", colorRed, err, colorReset)
		return nil
	}

	fmt.Printf("%sAI:%s ", colorPurple, colorReset)
	printStreamingResponse(aiResponse)
	conversation.addMessage("assistant", aiResponse)

	fmt.Println()
	return nil
}

func handleSaveCommand(conversation *Conversation) error {
	if err := saveConversation(conversation); err != nil {
		fmt.Printf("%sError saving conversation: %v%s\n", colorRed, err, colorReset)
	}
	return nil
}

func handleLoadCommand(userInput string, conversation *Conversation) error {
	parts := strings.SplitN(userInput, " ", 2)
	if len(parts) != 2 {
		fmt.Printf("%sUsage: /load <filename>%s\n", colorYellow, colorReset)
		return nil
	}
	loadedConversation, err := loadConversation(parts[1])
	if err != nil {
		fmt.Printf("%sError loading conversation: %v%s\n", colorRed, err, colorReset)
		return nil
	}
	conversation.replaceWith(loadedConversation)
	printConversationSummary(conversation)
	return nil
}

func getAIResponseWithRetry(ctx context.Context, apiClient *APIClient, conversation *Conversation) (string, error) {
	var (
		aiResponse string
		err        error
		backoff    = initialBackoff
	)

	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-apiClient.rateLimiter.C:
		case <-ctx.Done():
			return "", ctx.Err()
		}

		aiResponse, err = getAIResponse(ctx, apiClient, conversation)
		if err == nil {
			return aiResponse, nil
		}

		log.Printf("Attempt %d failed: %v", attempt+1, err)

		if attempt < maxRetries-1 {
			jitter := time.Duration(rand.Int63n(int64(backoff)))
			sleepTime := backoff + jitter
			log.Printf("Retrying in %v", sleepTime)
			time.Sleep(sleepTime)
			backoff *= time.Duration(backoffFactor)
		}
	}

	return "", fmt.Errorf("failed after %d attempts, last error: %w", maxRetries, err)
}

func getUserInput(scanner *bufio.Scanner) string {
	fmt.Printf("%sYou:%s ", colorGreen, colorReset)
	if !scanner.Scan() {
		return exitCommand
	}
	return strings.TrimSpace(scanner.Text())
}

func (c *Conversation) addMessage(role, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	tokens := len(strings.Fields(content))
	c.tokenCount += tokens
	c.History = append(c.History, Message{Role: role, Content: content, Timestamp: time.Now()})
	c.truncateHistory()
}

func (c *Conversation) truncateHistory() {
	for c.tokenCount > maxConversationTokens && len(c.History) > 1 {
		removedTokens := len(strings.Fields(c.History[1].Content))
		c.tokenCount -= removedTokens
		c.History = c.History[2:]
	}
}

func (c *Conversation) getHistory() []Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]Message(nil), c.History...)
}

func getAIResponse(ctx context.Context, apiClient *APIClient, conversation *Conversation) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	response, err := apiClient.sendRequest(ctx, conversation)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", response.StatusCode, string(body))
	}

	return processStreamResponse(response.Body)
}

func (c *APIClient) sendRequest(ctx context.Context, conversation *Conversation) (*http.Response, error) {
	truncatedHistory := truncateConversation(conversation.getHistory(), maxTokens)
	requestBody, err := createRequestBody(truncatedHistory)
	if err != nil {
		return nil, fmt.Errorf("failed to create request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", "AIChat/1.0")

	return c.httpClient.Do(req)
}

func truncateConversation(history []Message, maxTokens int) []Message {
	var truncated []Message
	totalTokens := 0

	for i := len(history) - 1; i >= 0; i-- {
		message := history[i]
		tokens := len(strings.Fields(message.Content))
		if totalTokens+tokens > maxTokens {
			break
		}
		totalTokens += tokens
		truncated = append([]Message{message}, truncated...)
	}

	return truncated
}

func createRequestBody(truncatedHistory []Message) ([]byte, error) {
	currentTime := time.Now()
	systemMessage := fmt.Sprintf("Current date and time: %s", currentTime.Format(time.RFC3339))

	apiMessages := []APIMessage{
		{Role: "system", Content: systemMessage},
	}

	for _, msg := range truncatedHistory {
		apiMessages = append(apiMessages, APIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	body := map[string]interface{}{
		"messages":    apiMessages,
		"model":       "llama-3.1-70b-versatile",
		"temperature": 0.7,
		"max_tokens":  maxTokens,
		"top_p":       0.9,
		"stream":      true,
		"stop":        []string{"\n\nHuman:", "\n\nAssistant:"},
	}

	return json.Marshal(body)
}

func processStreamResponse(body io.Reader) (string, error) {
	scanner := bufio.NewScanner(body)
	var buffer strings.Builder
	var lastError error

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var jsonResponse map[string]interface{}
		if err := json.Unmarshal([]byte(data), &jsonResponse); err != nil {
			lastError = err
			continue
		}

		if content := extractContent(jsonResponse); content != "" {
			buffer.WriteString(content)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read stream: %w", err)
	}

	if lastError != nil {
		return "", fmt.Errorf("error processing stream: %w", lastError)
	}

	return strings.TrimSpace(buffer.String()), nil
}

func extractContent(jsonResponse map[string]interface{}) string {
	choices, ok := jsonResponse["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return ""
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return ""
	}

	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		return ""
	}

	content, ok := delta["content"].(string)
	if !ok {
		return ""
	}

	return content
}

func clearScreen() {
	fmt.Print("\033[2J\033[H")
}

func printStreamingResponse(response string) {
	words := strings.Fields(response)
	for i, word := range words {
		fmt.Print(word)
		if i < len(words)-1 {
			fmt.Print(" ")
		}
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Println()
}

func saveConversation(conversation *Conversation) error {
	filename := fmt.Sprintf("conversation_%s.json", time.Now().Format("20060102_150405"))
	data, err := json.MarshalIndent(conversation.getHistory(), "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal conversation: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write conversation file: %w", err)
	}

	fmt.Printf("%sConversation saved to %s%s\n", colorGreen, filename, colorReset)
	return nil
}

func loadConversation(filename string) (*Conversation, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read conversation file: %w", err)
	}

	var history []Message
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("failed to unmarshal conversation: %w", err)
	}

	conversation := &Conversation{History: history}
	conversation.tokenCount = countTokens(history)
	return conversation, nil
}

func countTokens(messages []Message) int {
	count := 0
	for _, msg := range messages {
		count += len(strings.Fields(msg.Content))
	}
	return count
}

func printConversationSummary(conversation *Conversation) {
	fmt.Printf("%sConversation Summary:%s\n", colorCyan, colorReset)
	fmt.Printf("Total messages: %d\n", len(conversation.History))
	fmt.Printf("Total tokens: %d\n", conversation.tokenCount)
	fmt.Println("Last 3 exchanges:")
	for i := max(0, len(conversation.History)-6); i < len(conversation.History); i++ {
		msg := conversation.History[i]
		fmt.Printf("%s%s:%s %s\n", colorYellow, msg.Role, colorReset, truncateString(msg.Content, 50))
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (c *Conversation) replaceWith(other *Conversation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.History = other.History
	c.tokenCount = other.tokenCount
}

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
)
