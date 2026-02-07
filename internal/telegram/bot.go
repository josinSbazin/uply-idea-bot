package telegram

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/josinSbazin/idea-bot/internal/config"
	"github.com/josinSbazin/idea-bot/internal/domain/model"
	"github.com/josinSbazin/idea-bot/internal/domain/service"
)

type Bot struct {
	api           *tgbotapi.BotAPI
	ideaService   *service.IdeaService
	allowedGroups map[int64]bool
}

func NewBot(ideaService *service.IdeaService) (*Bot, error) {
	cfg := config.Get()

	api, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}
	api.Debug = true

	// Build allowed groups map for O(1) lookup
	allowedGroups := make(map[int64]bool)
	for _, groupID := range cfg.Telegram.AllowedGroups {
		allowedGroups[groupID] = true
	}

	log.Printf("Telegram bot authorized as @%s", api.Self.UserName)
	log.Printf("Allowed groups: %v", cfg.Telegram.AllowedGroups)

	return &Bot{
		api:           api,
		ideaService:   ideaService,
		allowedGroups: allowedGroups,
	}, nil
}

// Start begins polling for updates
func (b *Bot) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	log.Println("Telegram bot started, waiting for messages...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Telegram bot stopping...")
			b.api.StopReceivingUpdates()
			return ctx.Err()
		case update := <-updates:
			go b.handleUpdate(ctx, update)
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	log.Printf("Received update: %+v", update.UpdateID)

	if update.Message == nil {
		log.Printf("Update has no message, skipping")
		return
	}

	log.Printf("Message from chat %d (%s): %s", update.Message.Chat.ID, update.Message.Chat.Title, update.Message.Text)

	// Only process from allowed groups (if list is configured)
	if len(b.allowedGroups) > 0 && !b.allowedGroups[update.Message.Chat.ID] {
		log.Printf("Ignored message from unauthorized chat: %d (%s)",
			update.Message.Chat.ID, update.Message.Chat.Title)
		return
	}

	// Only process /idea command
	if !update.Message.IsCommand() {
		return
	}

	switch update.Message.Command() {
	case "idea":
		b.handleIdeaCommand(ctx, update.Message)
	case "start", "help":
		b.handleHelpCommand(update.Message)
	}
}

func (b *Bot) handleIdeaCommand(ctx context.Context, msg *tgbotapi.Message) {
	ideaText := strings.TrimSpace(msg.CommandArguments())

	if ideaText == "" {
		b.reply(msg, "❌ Пожалуйста, укажите текст идеи после команды.\n\nПример: `/idea добавить тёмную тему в консоль`")
		return
	}

	if len(ideaText) < 10 {
		b.reply(msg, "❌ Идея слишком короткая. Опишите её подробнее (минимум 10 символов).")
		return
	}

	if len(ideaText) > 2000 {
		b.reply(msg, "❌ Идея слишком длинная (максимум 2000 символов).")
		return
	}

	// Send "thinking" message
	thinkingMsg := b.reply(msg, "🤔 Анализирую идею...")

	// Get username
	username := msg.From.UserName
	if username == "" {
		username = msg.From.FirstName
	}

	// Create and enrich the idea
	input := model.CreateIdeaInput{
		TelegramMessageID: int64(msg.MessageID),
		TelegramChatID:    msg.Chat.ID,
		TelegramUserID:    msg.From.ID,
		TelegramUsername:  msg.From.UserName,
		TelegramFirstName: msg.From.FirstName,
		RawText:           ideaText,
	}

	// Use timeout context for Claude API
	enrichCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	idea, enriched, err := b.ideaService.CreateAndEnrich(enrichCtx, input)
	if err != nil {
		cfg := config.Get()

		// Check for duplicate error
		var dupErr *service.DuplicateError
		if errors.As(err, &dupErr) {
			existingURL := fmt.Sprintf("%s/ideas/%d", cfg.Web.BaseURL, dupErr.SimilarID)
			response := fmt.Sprintf("🔄 *Похожая идея уже существует\\!*\n\n"+
				"📝 %s\n\n"+
				"👉 [Идея \\#%d](%s)",
				escapeMarkdownV2(dupErr.Reason),
				dupErr.SimilarID,
				escapeMarkdownV2(existingURL))
			b.editMessageMarkdown(thinkingMsg, response)
			return
		}

		if strings.Contains(err.Error(), "rate limit") {
			b.editMessage(thinkingMsg, "⚠️ Слишком много идей за последний час. Попробуйте позже.")
		} else {
			log.Printf("Error creating idea: %v", err)
			b.editMessage(thinkingMsg, "❌ Произошла ошибка при сохранении идеи. Попробуйте позже.")
		}
		return
	}

	log.Printf("Idea %d created, enriched=%v", idea.ID, enriched != nil)

	// Format response
	cfg := config.Get()
	ideaURL := fmt.Sprintf("%s/ideas/%d", cfg.Web.BaseURL, idea.ID)

	var response string
	if enriched != nil {
		log.Printf("Formatting enriched response for idea %d", idea.ID)
		response = service.FormatEnrichedForTelegram(enriched)
		response += fmt.Sprintf("\n\n💾 [Идея \\#%d](%s) сохранена", idea.ID, escapeMarkdownV2(ideaURL))
		log.Printf("Formatted response length: %d chars", len(response))
	} else {
		response = fmt.Sprintf("💾 [Идея \\#%d](%s) сохранена\\!\n\n📝 %s\n\n_\\(Автоматический анализ недоступен\\)_",
			idea.ID, escapeMarkdownV2(ideaURL), escapeMarkdownV2(ideaText))
	}

	log.Printf("Sending edited message for idea %d", idea.ID)
	b.editMessageMarkdown(thinkingMsg, response)
	log.Printf("Edit message sent for idea %d", idea.ID)
}

func (b *Bot) handleHelpCommand(msg *tgbotapi.Message) {
	help := `🤖 *Idea Bot*

Bot for collecting and analyzing feature ideas with AI\.

*Commands:*
/idea <text> \- Submit a new idea
/help \- Show this help

*Example:*
\` + "`" + `/idea Add Slack integration for build notifications\` + "`" + `

Your idea will be analyzed by AI and saved for review\.`

	b.replyMarkdown(msg, help)
}

func (b *Bot) reply(msg *tgbotapi.Message, text string) *tgbotapi.Message {
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ReplyToMessageID = msg.MessageID

	sent, err := b.api.Send(reply)
	if err != nil {
		log.Printf("Failed to send message: %v", err)
		return nil
	}
	return &sent
}

func (b *Bot) replyMarkdown(msg *tgbotapi.Message, text string) *tgbotapi.Message {
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ReplyToMessageID = msg.MessageID
	reply.ParseMode = tgbotapi.ModeMarkdownV2

	sent, err := b.api.Send(reply)
	if err != nil {
		log.Printf("Failed to send markdown message: %v", err)
		// Fallback to plain text
		return b.reply(msg, stripMarkdown(text))
	}
	return &sent
}

func (b *Bot) editMessage(msg *tgbotapi.Message, text string) {
	if msg == nil {
		return
	}
	edit := tgbotapi.NewEditMessageText(msg.Chat.ID, msg.MessageID, text)
	if _, err := b.api.Send(edit); err != nil {
		log.Printf("Failed to edit message: %v", err)
	}
}

func (b *Bot) editMessageMarkdown(msg *tgbotapi.Message, text string) {
	if msg == nil {
		return
	}
	edit := tgbotapi.NewEditMessageText(msg.Chat.ID, msg.MessageID, text)
	edit.ParseMode = tgbotapi.ModeMarkdownV2

	if _, err := b.api.Send(edit); err != nil {
		log.Printf("Failed to edit markdown message: %v, trying plain text", err)
		// Fallback to plain text
		b.editMessage(msg, stripMarkdown(text))
	}
}

// escapeMarkdownV2 escapes special characters for Telegram MarkdownV2
func escapeMarkdownV2(text string) string {
	specialChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	result := text
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}
	return result
}

// stripMarkdown removes markdown formatting for fallback
func stripMarkdown(text string) string {
	// Remove escape characters
	result := strings.ReplaceAll(text, "\\", "")
	// Remove formatting markers
	result = strings.ReplaceAll(result, "*", "")
	result = strings.ReplaceAll(result, "_", "")
	result = strings.ReplaceAll(result, "`", "")
	return result
}
