package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/NotHarshhaa/aws-ghost/pkg/types"
)

// WebhookNotifier handles sending notifications to various webhook endpoints
type WebhookNotifier struct {
	SlackWebhookURL string
	TeamsWebhookURL string
	DiscordWebhookURL string
}

// SlackMessage represents a Slack webhook message
type SlackMessage struct {
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Attachment represents a Slack message attachment
type Attachment struct {
	Color     string  `json:"color"`
	Title     string  `json:"title"`
	Text      string  `json:"text"`
	Fields    []Field `json:"fields,omitempty"`
	Timestamp int64   `json:"ts"`
}

// Field represents a Slack message field
type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// TeamsMessage represents a Microsoft Teams webhook message
type TeamsMessage struct {
	Type       string    `json:"@type"`
	Context    string    `json:"@context"`
	ThemeColor string    `json:"themeColor"`
	Summary    string    `json:"summary"`
	Sections   []Section `json:"sections"`
}

// Section represents a Teams message section
type Section struct {
	ActivityTitle    string   `json:"activityTitle"`
	ActivitySubtitle string   `json:"activitySubtitle"`
	Facts            []Fact   `json:"facts"`
	Markdown         bool     `json:"markdown"`
}

// Fact represents a Teams message fact
type Fact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// DiscordMessage represents a Discord webhook message
type DiscordMessage struct {
	Username  string         `json:"username"`
	AvatarURL string         `json:"avatar_url"`
	Content   string         `json:"content"`
	Embeds    []DiscordEmbed `json:"embeds,omitempty"`
}

// DiscordEmbed represents a Discord message embed
type DiscordEmbed struct {
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Color       int          `json:"color"`
	Fields      []DiscordField `json:"fields,omitempty"`
	Timestamp   string       `json:"timestamp"`
}

// DiscordField represents a Discord embed field
type DiscordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// NewWebhookNotifier creates a new webhook notifier
func NewWebhookNotifier(slackURL, teamsURL, discordURL string) *WebhookNotifier {
	return &WebhookNotifier{
		SlackWebhookURL:   slackURL,
		TeamsWebhookURL:   teamsURL,
		DiscordWebhookURL: discordURL,
	}
}

// SendSlackNotification sends a notification to Slack
func (w *WebhookNotifier) SendSlackNotification(results []types.ScanResult) error {
	if w.SlackWebhookURL == "" {
		return fmt.Errorf("Slack webhook URL not configured")
	}

	var totalCost float64
	var totalResources int
	resourceCounts := make(map[string]int)

	for _, result := range results {
		totalCost += result.TotalCost
		totalResources += len(result.Resources)
		
		for _, resource := range result.Resources {
			resourceCounts[resource.Type]++
		}
	}

	// Determine message color based on waste amount
	color := "good" // green
	if totalCost > 100 {
		color = "warning" // yellow
	}
	if totalCost > 500 {
		color = "danger" // red
	}

	// Create fields for the message
	var fields []Field
	fields = append(fields, Field{
		Title: "Total Waste",
		Value: fmt.Sprintf("$%.2f/month", totalCost),
		Short: true,
	})
	fields = append(fields, Field{
		Title: "Ghost Resources",
		Value: fmt.Sprintf("%d", totalResources),
		Short: true,
	})

	// Add resource type breakdown
	for resType, count := range resourceCounts {
		if count > 0 {
			fields = append(fields, Field{
				Title: resType,
				Value: fmt.Sprintf("%d resources", count),
				Short: true,
			})
		}
	}

	attachment := Attachment{
		Color:     color,
		Title:     "👻 AWS Ghost Scan Results",
		Text:      fmt.Sprintf("Found %d ghost resources costing $%.2f/month", totalResources, totalCost),
		Fields:    fields,
		Timestamp: time.Now().Unix(),
	}

	message := SlackMessage{
		Text:        "AWS Ghost Scan Complete",
		Attachments: []Attachment{attachment},
	}

	return w.sendWebhook(w.SlackWebhookURL, message)
}

// SendTeamsNotification sends a notification to Microsoft Teams
func (w *WebhookNotifier) SendTeamsNotification(results []types.ScanResult) error {
	if w.TeamsWebhookURL == "" {
		return fmt.Errorf("Teams webhook URL not configured")
	}

	var totalCost float64
	var totalResources int
	resourceCounts := make(map[string]int)

	for _, result := range results {
		totalCost += result.TotalCost
		totalResources += len(result.Resources)
		
		for _, resource := range result.Resources {
			resourceCounts[resource.Type]++
		}
	}

	// Determine theme color based on waste amount
	themeColor := "00FF00" // green
	if totalCost > 100 {
		themeColor = "FFFF00" // yellow
	}
	if totalCost > 500 {
		themeColor = "FF0000" // red
	}

	// Create facts for the message
	var facts []Fact
	facts = append(facts, Fact{
		Name:  "Total Waste",
		Value: fmt.Sprintf("$%.2f/month", totalCost),
	})
	facts = append(facts, Fact{
		Name:  "Ghost Resources",
		Value: fmt.Sprintf("%d", totalResources),
	})

	// Add resource type breakdown
	for resType, count := range resourceCounts {
		if count > 0 {
			facts = append(facts, Fact{
				Name:  resType,
				Value: fmt.Sprintf("%d resources", count),
			})
		}
	}

	section := Section{
		ActivityTitle:    "👻 AWS Ghost Scan Results",
		ActivitySubtitle: fmt.Sprintf("Found %d ghost resources costing $%.2f/month", totalResources, totalCost),
		Facts:            facts,
		Markdown:         true,
	}

	message := TeamsMessage{
		Type:       "MessageCard",
		Context:    "http://schema.org/extensions",
		ThemeColor: themeColor,
		Summary:    "AWS Ghost Scan Complete",
		Sections:   []Section{section},
	}

	return w.sendWebhook(w.TeamsWebhookURL, message)
}

// SendDiscordNotification sends a notification to Discord
func (w *WebhookNotifier) SendDiscordNotification(results []types.ScanResult) error {
	if w.DiscordWebhookURL == "" {
		return fmt.Errorf("Discord webhook URL not configured")
	}

	var totalCost float64
	var totalResources int
	resourceCounts := make(map[string]int)

	for _, result := range results {
		totalCost += result.TotalCost
		totalResources += len(result.Resources)
		
		for _, resource := range result.Resources {
			resourceCounts[resource.Type]++
		}
	}

	// Determine embed color based on waste amount
	color := 0x00FF00 // green
	if totalCost > 100 {
		color = 0xFFFF00 // yellow
	}
	if totalCost > 500 {
		color = 0xFF0000 // red
	}

	// Create fields for the embed
	var fields []DiscordField
	fields = append(fields, DiscordField{
		Name:   "Total Waste",
		Value:  fmt.Sprintf("$%.2f/month", totalCost),
		Inline: true,
	})
	fields = append(fields, DiscordField{
		Name:   "Ghost Resources",
		Value:  fmt.Sprintf("%d", totalResources),
		Inline: true,
	})

	// Add resource type breakdown
	for resType, count := range resourceCounts {
		if count > 0 {
			fields = append(fields, DiscordField{
				Name:   resType,
				Value:  fmt.Sprintf("%d resources", count),
				Inline: true,
			})
		}
	}

	embed := DiscordEmbed{
		Title:       "👻 AWS Ghost Scan Results",
		Description: fmt.Sprintf("Found %d ghost resources costing $%.2f/month", totalResources, totalCost),
		Color:       color,
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	message := DiscordMessage{
		Username: "AWS Ghost",
		Content:  "AWS Ghost Scan Complete",
		Embeds:   []DiscordEmbed{embed},
	}

	return w.sendWebhook(w.DiscordWebhookURL, message)
}

// SendTrendAlert sends a trend alert notification
func (w *WebhookNotifier) SendTrendAlert(currentWaste, previousWaste float64, changePercent float64) error {
	title := "📈 AWS Ghost Trend Alert"
	
	var message string
	var color string
	
	if changePercent > 20 {
		message = fmt.Sprintf("⚠️ Waste increased by %.1f%% ($%.2f → $%.2f)", changePercent, previousWaste, currentWaste)
		color = "danger"
	} else if changePercent < -20 {
		message = fmt.Sprintf("✅ Waste decreased by %.1f%% ($%.2f → $%.2f)", -changePercent, previousWaste, currentWaste)
		color = "good"
	} else {
		message = fmt.Sprintf("📊 Waste changed by %.1f%% ($%.2f → $%.2f)", changePercent, previousWaste, currentWaste)
		color = "warning"
	}

	// Send to all configured platforms
	var errors []error

	if w.SlackWebhookURL != "" {
		attachment := Attachment{
			Color:     color,
			Title:     title,
			Text:      message,
			Timestamp: time.Now().Unix(),
		}
		
		slackMsg := SlackMessage{
			Text:        title,
			Attachments: []Attachment{attachment},
		}
		
		if err := w.sendWebhook(w.SlackWebhookURL, slackMsg); err != nil {
			errors = append(errors, fmt.Errorf("Slack: %w", err))
		}
	}

	if w.TeamsWebhookURL != "" {
		themeColor := "00FF00" // green
		if color == "danger" {
			themeColor = "FF0000" // red
		} else if color == "warning" {
			themeColor = "FFFF00" // yellow
		}

		section := Section{
			ActivityTitle:    title,
			ActivitySubtitle: message,
			Markdown:         true,
		}

		teamsMsg := TeamsMessage{
			Type:       "MessageCard",
			Context:    "http://schema.org/extensions",
			ThemeColor: themeColor,
			Summary:    title,
			Sections:   []Section{section},
		}

		if err := w.sendWebhook(w.TeamsWebhookURL, teamsMsg); err != nil {
			errors = append(errors, fmt.Errorf("Teams: %w", err))
		}
	}

	if w.DiscordWebhookURL != "" {
		embedColor := 0x00FF00 // green
		if color == "danger" {
			embedColor = 0xFF0000 // red
		} else if color == "warning" {
			embedColor = 0xFFFF00 // yellow
		}

		embed := DiscordEmbed{
			Title:       title,
			Description: message,
			Color:       embedColor,
			Timestamp:   time.Now().Format(time.RFC3339),
		}

		discordMsg := DiscordMessage{
			Username: "AWS Ghost",
			Content:  title,
			Embeds:   []DiscordEmbed{embed},
		}

		if err := w.sendWebhook(w.DiscordWebhookURL, discordMsg); err != nil {
			errors = append(errors, fmt.Errorf("Discord: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("webhook errors: %v", errors)
	}

	return nil
}

// sendWebhook sends a webhook request
func (w *WebhookNotifier) sendWebhook(url string, payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// TestWebhooks tests all configured webhook endpoints
func (w *WebhookNotifier) TestWebhooks() error {
	title := "🧪 AWS Ghost Webhook Test"
	message := "This is a test message from AWS Ghost to verify webhook connectivity."

	var errors []error

	if w.SlackWebhookURL != "" {
		attachment := Attachment{
			Color:     "good",
			Title:     title,
			Text:      message,
			Timestamp: time.Now().Unix(),
		}
		
		slackMsg := SlackMessage{
			Text:        title,
			Attachments: []Attachment{attachment},
		}
		
		if err := w.sendWebhook(w.SlackWebhookURL, slackMsg); err != nil {
			errors = append(errors, fmt.Errorf("Slack: %w", err))
		}
	}

	if w.TeamsWebhookURL != "" {
		section := Section{
			ActivityTitle:    title,
			ActivitySubtitle: message,
			Markdown:         true,
		}

		teamsMsg := TeamsMessage{
			Type:       "MessageCard",
			Context:    "http://schema.org/extensions",
			ThemeColor: "00FF00",
			Summary:    title,
			Sections:   []Section{section},
		}

		if err := w.sendWebhook(w.TeamsWebhookURL, teamsMsg); err != nil {
			errors = append(errors, fmt.Errorf("Teams: %w", err))
		}
	}

	if w.DiscordWebhookURL != "" {
		embed := DiscordEmbed{
			Title:       title,
			Description: message,
			Color:       0x00FF00,
			Timestamp:   time.Now().Format(time.RFC3339),
		}

		discordMsg := DiscordMessage{
			Username: "AWS Ghost",
			Content:  title,
			Embeds:   []DiscordEmbed{embed},
		}

		if err := w.sendWebhook(w.DiscordWebhookURL, discordMsg); err != nil {
			errors = append(errors, fmt.Errorf("Discord: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("webhook test errors: %v", errors)
	}

	return nil
}
