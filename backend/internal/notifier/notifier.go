package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rafael/monitor-de-rede-backend/internal/models"
)

type AlertPayload struct {
	Monitor   models.Monitor     `json:"monitor"`
	OldStatus models.MonitorStatus `json:"old_status"`
	NewStatus models.MonitorStatus `json:"new_status"`
	Message   string             `json:"message"`
	Timestamp time.Time          `json:"timestamp"`
}

func Send(ctx context.Context, wh models.Webhook, alert AlertPayload) error {
	client := &http.Client{Timeout: 8 * time.Second}

	switch wh.Type {
	case models.WebhookDiscord:
		return sendDiscord(ctx, client, wh.URL, alert)
	case models.WebhookTelegram:
		return sendTelegram(ctx, client, wh.BotToken, wh.ChatID, alert)
	case models.WebhookSlack:
		return sendSlack(ctx, client, wh.URL, alert)
	case models.WebhookGeneric:
		return sendGeneric(ctx, client, wh.URL, alert)
	default:
		return fmt.Errorf("unsupported webhook type: %s", wh.Type)
	}
}

func sendDiscord(ctx context.Context, client *http.Client, webhookURL string, alert AlertPayload) error {
	if webhookURL == "" {
		return fmt.Errorf("discord webhook URL is empty")
	}

	color := 0x22c55e // green
	emoji := "✅"
	title := fmt.Sprintf("%s [%s] is UP", emoji, alert.Monitor.Name)

	switch alert.NewStatus {
	case models.StatusDown:
		color = 0xef4444
		emoji = "🔴"
		title = fmt.Sprintf("%s [%s] is DOWN", emoji, alert.Monitor.Name)
	case models.StatusDegraded:
		color = 0xf59e0b
		emoji = "⚠️"
		title = fmt.Sprintf("%s [%s] is DEGRADED", emoji, alert.Monitor.Name)
	}

	payload := map[string]any{
		"username": "NetPulse Monitor",
		"embeds": []map[string]any{
			{
				"title":       title,
				"description": alert.Message,
				"color":       color,
				"fields": []map[string]any{
					{"name": "Type", "value": string(alert.Monitor.Type), "inline": true},
					{"name": "Target", "value": alert.Monitor.Target, "inline": true},
					{"name": "Latency", "value": fmt.Sprintf("%d ms", alert.Monitor.LastLatencyMs), "inline": true},
				},
				"timestamp": alert.Timestamp.Format(time.RFC3339),
			},
		},
	}

	return postJSON(ctx, client, webhookURL, payload)
}

func sendTelegram(ctx context.Context, client *http.Client, botToken, chatID string, alert AlertPayload) error {
	if botToken == "" || chatID == "" {
		return fmt.Errorf("telegram bot_token and chat_id are required")
	}

	emoji := "✅"
	if alert.NewStatus == models.StatusDown {
		emoji = "🔴"
	} else if alert.NewStatus == models.StatusDegraded {
		emoji = "⚠️"
	}

	text := fmt.Sprintf(
		"%s *NetPulse Alert*\n*Service:* %s (%s)\n*Target:* `%s`\n*Status:* %s → *%s*\n*Details:* %s",
		emoji,
		alert.Monitor.Name,
		alert.Monitor.Type,
		alert.Monitor.Target,
		alert.OldStatus,
		alert.NewStatus,
		alert.Message,
	)

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	return postJSON(ctx, client, endpoint, payload)
}

func sendSlack(ctx context.Context, client *http.Client, webhookURL string, alert AlertPayload) error {
	if webhookURL == "" {
		return fmt.Errorf("slack webhook URL is empty")
	}

	color := "#22c55e"
	emoji := ":white_check_mark:"
	if alert.NewStatus == models.StatusDown {
		color = "#ef4444"
		emoji = ":red_circle:"
	} else if alert.NewStatus == models.StatusDegraded {
		color = "#f59e0b"
		emoji = ":warning:"
	}

	payload := map[string]any{
		"attachments": []map[string]any{
			{
				"color": color,
				"title": fmt.Sprintf("%s [%s] is %s", emoji, alert.Monitor.Name, strings.ToUpper(string(alert.NewStatus))),
				"text":  alert.Message,
				"fields": []map[string]any{
					{"title": "Type", "value": string(alert.Monitor.Type), "short": true},
					{"title": "Target", "value": alert.Monitor.Target, "short": true},
					{"title": "Latency", "value": fmt.Sprintf("%d ms", alert.Monitor.LastLatencyMs), "short": true},
				},
				"ts": alert.Timestamp.Unix(),
			},
		},
	}

	return postJSON(ctx, client, webhookURL, payload)
}

func sendGeneric(ctx context.Context, client *http.Client, targetURL string, alert AlertPayload) error {
	if targetURL == "" {
		return fmt.Errorf("webhook URL is empty")
	}
	return postJSON(ctx, client, targetURL, alert)
}

func postJSON(ctx context.Context, client *http.Client, endpoint string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}
