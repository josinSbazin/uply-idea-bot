package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/josinSbazin/idea-bot/internal/config"
	"github.com/josinSbazin/idea-bot/internal/domain/model"
)

// defaultSystemPrompt is used when no custom prompt file is provided
const defaultSystemPrompt = `You are an AI assistant specialized in analyzing feature ideas for software projects.

## Your Task

Analyze the raw idea submitted by a team member and provide a structured, enriched version that can be used for planning and prioritization.

Guidelines:
- Be constructive and helpful
- If the idea is vague, make reasonable assumptions
- Provide actionable acceptance criteria
- Consider technical implications and potential risks
- Suggest which components might be affected

Always respond in the same language as the original idea.
Return ONLY valid JSON without any markdown formatting or code blocks.`

const responseSchema = `{
  "type": "object",
  "properties": {
    "title": {
      "type": "string",
      "description": "Short idea title (up to 100 characters)"
    },
    "summary": {
      "type": "string",
      "description": "Brief description in 1-2 sentences"
    },
    "detailed_description": {
      "type": "string",
      "description": "Detailed description of the functionality"
    },
    "category": {
      "type": "string",
      "enum": ["feature", "improvement", "bug", "integration", "other"],
      "description": "Category: feature (new feature), improvement (enhancement), bug (bug fix), integration (external service integration), other"
    },
    "priority": {
      "type": "string",
      "enum": ["low", "medium", "high", "critical"],
      "description": "Priority based on potential value for users"
    },
    "complexity": {
      "type": "string",
      "enum": ["trivial", "small", "medium", "large", "epic"],
      "description": "Complexity estimate: trivial (< 1 hour), small (1-4 hours), medium (1-3 days), large (1-2 weeks), epic (> 2 weeks)"
    },
    "affected_components": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Which components/modules this idea affects"
    },
    "user_story": {
      "type": "string",
      "description": "User story in format: As a [role], I want [action], so that [goal]"
    },
    "acceptance_criteria": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Acceptance criteria - what should work to consider the task complete"
    },
    "technical_notes": {
      "type": "string",
      "description": "Technical notes and implementation recommendations"
    },
    "related_features": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Related existing features to integrate with"
    },
    "potential_risks": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Potential risks and implementation challenges"
    }
  },
  "required": ["title", "summary", "detailed_description", "category", "priority", "complexity", "user_story", "acceptance_criteria"]
}`

type ClaudeService struct {
	client       anthropic.Client
	model        string
	systemPrompt string
}

func NewClaudeService() *ClaudeService {
	cfg := config.Get()
	client := anthropic.NewClient(option.WithAPIKey(cfg.Claude.APIKey))

	// Load system prompt from file or use default
	systemPrompt := defaultSystemPrompt
	if cfg.Claude.SystemPromptFile != "" {
		data, err := os.ReadFile(cfg.Claude.SystemPromptFile)
		if err != nil {
			log.Printf("Warning: failed to load system prompt from %s: %v, using default", cfg.Claude.SystemPromptFile, err)
		} else {
			systemPrompt = string(data)
			log.Printf("Loaded custom system prompt from %s", cfg.Claude.SystemPromptFile)
		}
	}

	return &ClaudeService{
		client:       client,
		model:        cfg.Claude.Model,
		systemPrompt: systemPrompt,
	}
}

// EnrichIdea sends the raw idea to Claude and returns structured analysis
func (s *ClaudeService) EnrichIdea(ctx context.Context, rawIdea string, username string) (*model.EnrichedIdea, error) {
	userPrompt := fmt.Sprintf(`User @%s submitted an idea:

"%s"

Analyze this idea and return a structured JSON according to the schema.
Do not use markdown formatting, return only clean JSON.`, username, rawIdea)

	message, err := s.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     s.model,
		MaxTokens: 2000,
		System: []anthropic.TextBlockParam{
			{Text: s.systemPrompt + "\n\nExpected JSON schema:\n" + responseSchema},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude API error: %w", err)
	}

	// Extract text from response
	var responseText string
	for _, block := range message.Content {
		if block.Type == "text" {
			responseText = block.Text
			break
		}
	}

	if responseText == "" {
		return nil, fmt.Errorf("empty response from Claude")
	}

	// Parse JSON response
	var enriched model.EnrichedIdea
	if err := json.Unmarshal([]byte(responseText), &enriched); err != nil {
		return nil, fmt.Errorf("failed to parse Claude response as JSON: %w\nResponse: %s", err, responseText)
	}

	return &enriched, nil
}

// FormatEnrichedForTelegram formats the enriched idea for Telegram message
func FormatEnrichedForTelegram(enriched *model.EnrichedIdea) string {
	msg := fmt.Sprintf("✨ *%s*\n\n", escapeMarkdown(enriched.Title))
	msg += fmt.Sprintf("📝 %s\n\n", escapeMarkdown(enriched.Summary))

	msg += fmt.Sprintf("📂 Категория: `%s`\n", enriched.Category)
	msg += fmt.Sprintf("⚡ Приоритет: `%s`\n", enriched.Priority)
	msg += fmt.Sprintf("📊 Сложность: `%s`\n", enriched.Complexity)

	if len(enriched.AffectedComponents) > 0 {
		msg += "📁 Components: "
		for i, repo := range enriched.AffectedComponents {
			if i > 0 {
				msg += ", "
			}
			msg += fmt.Sprintf("`%s`", repo)
		}
		msg += "\n"
	}

	msg += fmt.Sprintf("\n👤 *User Story:*\n%s\n", escapeMarkdown(enriched.UserStory))

	if len(enriched.AcceptanceCriteria) > 0 {
		msg += "\n✅ *Критерии приёмки:*\n"
		for _, criteria := range enriched.AcceptanceCriteria {
			msg += fmt.Sprintf("• %s\n", escapeMarkdown(criteria))
		}
	}

	if enriched.TechnicalNotes != "" {
		msg += fmt.Sprintf("\n🔧 *Технические заметки:*\n%s\n", escapeMarkdown(enriched.TechnicalNotes))
	}

	if len(enriched.PotentialRisks) > 0 {
		msg += "\n⚠️ *Риски:*\n"
		for _, risk := range enriched.PotentialRisks {
			msg += fmt.Sprintf("• %s\n", escapeMarkdown(risk))
		}
	}

	return msg
}

// escapeMarkdown escapes special characters for Telegram MarkdownV2
func escapeMarkdown(text string) string {
	specialChars := []string{"\\", "_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	result := text
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}
	return result
}

// DuplicateResult represents the result of duplicate check
type DuplicateResult struct {
	IsDuplicate   bool   `json:"is_duplicate"`
	SimilarIdeaID int64  `json:"similar_idea_id,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

// CheckDuplicate checks if a new idea is similar to existing ones
func (s *ClaudeService) CheckDuplicate(ctx context.Context, newIdea string, existingIdeas []model.IdeaSummary) (*DuplicateResult, error) {
	if len(existingIdeas) == 0 {
		return &DuplicateResult{IsDuplicate: false}, nil
	}

	// Build existing ideas list
	var ideasList strings.Builder
	for _, idea := range existingIdeas {
		title := idea.Title
		if title == "" {
			title = idea.RawText
			if len(title) > 100 {
				title = title[:100] + "..."
			}
		}
		ideasList.WriteString(fmt.Sprintf("- ID %d: %s\n", idea.ID, title))
	}

	prompt := fmt.Sprintf(`Проверь, является ли новая идея дубликатом или очень похожей на одну из существующих идей.

Новая идея:
"%s"

Существующие идеи:
%s

Верни JSON ответ:
- is_duplicate: true если новая идея по смыслу совпадает или очень похожа на существующую
- similar_idea_id: ID похожей идеи (только если is_duplicate=true)
- reason: краткое объяснение почему считаешь дубликатом (на русском)

Считай дубликатом только если идеи описывают одну и ту же функциональность или улучшение.
НЕ считай дубликатом если идеи просто в одной области но про разное.

Верни ТОЛЬКО JSON без markdown.`, newIdea, ideasList.String())

	message, err := s.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     s.model,
		MaxTokens: 500,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude API error: %w", err)
	}

	var responseText string
	for _, block := range message.Content {
		if block.Type == "text" {
			responseText = block.Text
			break
		}
	}

	if responseText == "" {
		return &DuplicateResult{IsDuplicate: false}, nil
	}

	var result DuplicateResult
	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		// If parsing fails, assume not a duplicate
		return &DuplicateResult{IsDuplicate: false}, nil
	}

	return &result, nil
}
