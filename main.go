package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Config struct {
	DiscordToken string
	APIKey       string
}

func loadConfig() (*Config, error) {
	discordToken := os.Getenv("DISCORD_TOKEN")
	apiKey := os.Getenv("API_KEY")

	if discordToken == "" || apiKey == "" {
		return nil, fmt.Errorf("API key or Discord token is not set in environment variables")
	}

	return &Config{
		DiscordToken: discordToken,
		APIKey:       apiKey,
	}, nil
}

func createDiscordSession(token string) (*discordgo.Session, error) {
	return discordgo.New("Bot " + token)
}

func handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, apiKey string) {
	if i.Type != discordgo.InteractionApplicationCommand || i.ApplicationCommandData().Name != "chat" {
		return
	}

	userInput := i.ApplicationCommandData().Options[0].StringValue()

	if err := respondToInteraction(s, i, userInput); err != nil {
		fmt.Println("Error responding to interaction:", err)
		return
	}

	responseContent, err := getAIResponse(apiKey, userInput)
	if err != nil {
		fmt.Println("Error getting AI response:", err)
		return
	}

	if err := editInteractionResponse(s, i, responseContent); err != nil {
		fmt.Println("Error editing interaction response:", err)
	}
}

func respondToInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, content string) error {
	initialResponse := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	}
	return s.InteractionRespond(i.Interaction, initialResponse)
}

func editInteractionResponse(s *discordgo.Session, i *discordgo.InteractionCreate, content string) error {
	if len(content) > 2000 {
		file := &discordgo.File{
			Name:        "response.txt",
			ContentType: "text/plain",
			Reader:      bytes.NewReader([]byte(content)),
		}
		msg := "Response is too long, sending as a file."
		_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
			Files:   []*discordgo.File{file},
		})
		return err
	}
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
	return err
}

func getAIResponse(apiKey, userInput string) (string, error) {
	url := "https://api.groq.com/openai/v1/chat/completions"
	client := &http.Client{Timeout: 5 * time.Minute}

	payload := map[string]interface{}{
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": userInput,
			},
		},
		"model": "llama3-70b-8192",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error marshalling JSON: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %w", err)
	}
	defer closeResponseBody(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status: %s", resp.Status)
	}

	return parseAIResponse(resp.Body)
}

func closeResponseBody(body io.ReadCloser) {
	if err := body.Close(); err != nil {
		fmt.Println("Error closing response body:", err)
	}
}

func parseAIResponse(body io.Reader) (string, error) {
	var result map[string]interface{}
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok {
					return content, nil
				}
			}
		}
	}

	return "", fmt.Errorf("invalid response format")
}

func main() {
	config, err := loadConfig()
	if err != nil {
		fmt.Println(err)
		return
	}

	dg, err := createDiscordSession(config.DiscordToken)
	if err != nil {
		fmt.Println(err)
		return
	}

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		handleInteraction(s, i, config.APIKey)
	})

	dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		if err := createApplicationCommand(s); err != nil {
			fmt.Println("Error creating application command:", err)
		}
	})

	if err := dg.Open(); err != nil {
		fmt.Println("Error opening Discord session:", err)
		return
	}

	fmt.Println("Bot is now running. Press CTRL+C to exit.")
	select {}
}

func createApplicationCommand(s *discordgo.Session) error {
	_, err := s.ApplicationCommandCreate(s.State.User.ID, "", &discordgo.ApplicationCommand{
		Name:        "chat",
		Description: "Chat with AI",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "message",
				Description: "Message to send to AI",
				Required:    true,
			},
		},
	})
	return err
}
