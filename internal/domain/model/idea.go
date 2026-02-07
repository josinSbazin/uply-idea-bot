package model

import (
	"encoding/json"
	"time"
)

type IdeaStatus string

const (
	StatusNew         IdeaStatus = "new"
	StatusReviewed    IdeaStatus = "reviewed"
	StatusAccepted    IdeaStatus = "accepted"
	StatusRejected    IdeaStatus = "rejected"
	StatusInProgress  IdeaStatus = "in_progress"
	StatusImplemented IdeaStatus = "implemented"
)

func (s IdeaStatus) Label() string {
	labels := map[IdeaStatus]string{
		StatusNew:         "Новая",
		StatusReviewed:    "Рассмотрена",
		StatusAccepted:    "Принята",
		StatusRejected:    "Отклонена",
		StatusInProgress:  "В работе",
		StatusImplemented: "Реализована",
	}
	if l, ok := labels[s]; ok {
		return l
	}
	return string(s)
}

type IdeaCategory string

const (
	CategoryFeature     IdeaCategory = "feature"
	CategoryImprovement IdeaCategory = "improvement"
	CategoryBug         IdeaCategory = "bug"
	CategoryIntegration IdeaCategory = "integration"
	CategoryOther       IdeaCategory = "other"
)

func (c IdeaCategory) Label() string {
	labels := map[IdeaCategory]string{
		CategoryFeature:     "Фича",
		CategoryImprovement: "Улучшение",
		CategoryBug:         "Баг",
		CategoryIntegration: "Интеграция",
		CategoryOther:       "Другое",
	}
	if l, ok := labels[c]; ok {
		return l
	}
	return string(c)
}

type IdeaPriority string

const (
	PriorityLow      IdeaPriority = "low"
	PriorityMedium   IdeaPriority = "medium"
	PriorityHigh     IdeaPriority = "high"
	PriorityCritical IdeaPriority = "critical"
)

func (p IdeaPriority) Label() string {
	labels := map[IdeaPriority]string{
		PriorityLow:      "Низкий",
		PriorityMedium:   "Средний",
		PriorityHigh:     "Высокий",
		PriorityCritical: "Критический",
	}
	if l, ok := labels[p]; ok {
		return l
	}
	return string(p)
}

type IdeaComplexity string

const (
	ComplexityTrivial IdeaComplexity = "trivial"
	ComplexitySmall   IdeaComplexity = "small"
	ComplexityMedium  IdeaComplexity = "medium"
	ComplexityLarge   IdeaComplexity = "large"
	ComplexityEpic    IdeaComplexity = "epic"
)

func (c IdeaComplexity) Label() string {
	labels := map[IdeaComplexity]string{
		ComplexityTrivial: "Тривиальная",
		ComplexitySmall:   "Маленькая",
		ComplexityMedium:  "Средняя",
		ComplexityLarge:   "Большая",
		ComplexityEpic:    "Эпик",
	}
	if l, ok := labels[c]; ok {
		return l
	}
	return string(c)
}

// EnrichedIdea represents the structured response from Claude
type EnrichedIdea struct {
	Title              string   `json:"title"`
	Summary            string   `json:"summary"`
	DetailedDesc       string   `json:"detailed_description"`
	Category           string   `json:"category"`
	Priority           string   `json:"priority"`
	Complexity         string   `json:"complexity"`
	AffectedComponents []string `json:"affected_components"`
	UserStory          string   `json:"user_story"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	TechnicalNotes     string   `json:"technical_notes,omitempty"`
	RelatedFeatures    []string `json:"related_features,omitempty"`
	PotentialRisks     []string `json:"potential_risks,omitempty"`
}

// Idea represents a feature idea
type Idea struct {
	ID                 int64          `json:"id"`
	TelegramMessageID  int64          `json:"telegram_message_id"`
	TelegramChatID     int64          `json:"telegram_chat_id"`
	TelegramUserID     int64          `json:"telegram_user_id"`
	TelegramUsername   string         `json:"telegram_username,omitempty"`
	TelegramFirstName  string         `json:"telegram_first_name,omitempty"`
	RawText            string         `json:"raw_text"`
	EnrichedJSON       string         `json:"enriched_json,omitempty"`
	Enriched           *EnrichedIdea  `json:"-"`
	Title              string         `json:"title,omitempty"`
	Category           IdeaCategory   `json:"category,omitempty"`
	Priority           IdeaPriority   `json:"priority,omitempty"`
	Complexity         IdeaComplexity `json:"complexity,omitempty"`
	AffectedComponents []string       `json:"affected_components,omitempty"`
	Status             IdeaStatus     `json:"status"`
	AdminNotes         string         `json:"admin_notes,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

// ParseEnriched parses the EnrichedJSON field into Enriched struct
func (i *Idea) ParseEnriched() error {
	if i.EnrichedJSON == "" {
		return nil
	}
	i.Enriched = &EnrichedIdea{}
	return json.Unmarshal([]byte(i.EnrichedJSON), i.Enriched)
}

// AffectedComponentsStr returns affected components as comma-separated string
func (i *Idea) AffectedComponentsStr() string {
	if len(i.AffectedComponents) == 0 {
		return ""
	}
	result := ""
	for idx, r := range i.AffectedComponents {
		if idx > 0 {
			result += ", "
		}
		result += r
	}
	return result
}

// CreateIdeaInput represents input for creating a new idea
type CreateIdeaInput struct {
	TelegramMessageID int64
	TelegramChatID    int64
	TelegramUserID    int64
	TelegramUsername  string
	TelegramFirstName string
	RawText           string
}

// IdeaFilter represents filters for listing ideas
type IdeaFilter struct {
	Status   []IdeaStatus
	Category []IdeaCategory
	Priority []IdeaPriority
	Limit    int
	Offset   int
}

// IdeaSummary is a lightweight representation of idea for duplicate checking
type IdeaSummary struct {
	ID      int64
	Title   string
	RawText string
}

// AllStatuses returns all possible statuses
func AllStatuses() []IdeaStatus {
	return []IdeaStatus{
		StatusNew,
		StatusReviewed,
		StatusAccepted,
		StatusRejected,
		StatusInProgress,
		StatusImplemented,
	}
}

// AllCategories returns all possible categories
func AllCategories() []IdeaCategory {
	return []IdeaCategory{
		CategoryFeature,
		CategoryImprovement,
		CategoryBug,
		CategoryIntegration,
		CategoryOther,
	}
}

// AllPriorities returns all possible priorities
func AllPriorities() []IdeaPriority {
	return []IdeaPriority{
		PriorityLow,
		PriorityMedium,
		PriorityHigh,
		PriorityCritical,
	}
}
