package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/josinSbazin/idea-bot/internal/config"
	"github.com/josinSbazin/idea-bot/internal/domain/model"
	"github.com/josinSbazin/idea-bot/internal/storage"
	"golang.org/x/time/rate"
)

type IdeaService struct {
	repo          *storage.IdeaRepository
	claudeService *ClaudeService
	rateLimiter   *RateLimiter
}

func NewIdeaService() *IdeaService {
	cfg := config.Get()
	return &IdeaService{
		repo:          storage.NewIdeaRepository(),
		claudeService: NewClaudeService(),
		rateLimiter:   NewRateLimiter(cfg.RateLimit.PerUser, cfg.RateLimit.Global),
	}
}

// DuplicateError represents a duplicate idea error
type DuplicateError struct {
	SimilarID int64
	Reason    string
}

func (e *DuplicateError) Error() string {
	return fmt.Sprintf("duplicate of idea #%d: %s", e.SimilarID, e.Reason)
}

// CreateAndEnrich creates a new idea and enriches it with Claude
func (s *IdeaService) CreateAndEnrich(ctx context.Context, input model.CreateIdeaInput) (*model.Idea, *model.EnrichedIdea, error) {
	log.Printf("CreateAndEnrich called for user %d: %s", input.TelegramUserID, input.RawText[:min(50, len(input.RawText))])

	// Check rate limit
	if !s.rateLimiter.Allow(input.TelegramUserID) {
		log.Printf("Rate limit exceeded for user %d", input.TelegramUserID)
		return nil, nil, fmt.Errorf("rate limit exceeded")
	}

	// Check for duplicates first
	log.Printf("Checking for duplicate ideas...")
	existingIdeas, err := s.repo.ListSummaries()
	if err != nil {
		log.Printf("Warning: failed to get existing ideas for duplicate check: %v", err)
	} else if len(existingIdeas) > 0 {
		dupResult, err := s.claudeService.CheckDuplicate(ctx, input.RawText, existingIdeas)
		if err != nil {
			log.Printf("Warning: duplicate check failed: %v", err)
		} else if dupResult != nil && dupResult.IsDuplicate {
			log.Printf("Duplicate found: idea #%d - %s", dupResult.SimilarIdeaID, dupResult.Reason)
			return nil, nil, &DuplicateError{
				SimilarID: dupResult.SimilarIdeaID,
				Reason:    dupResult.Reason,
			}
		}
	}
	log.Printf("No duplicates found, creating idea...")

	// Create the idea
	idea, err := s.repo.Create(input)
	if err != nil {
		log.Printf("Failed to create idea: %v", err)
		return nil, nil, fmt.Errorf("failed to create idea: %w", err)
	}
	log.Printf("Idea created with ID %d", idea.ID)

	// Enrich with Claude
	username := input.TelegramUsername
	if username == "" {
		username = input.TelegramFirstName
	}

	log.Printf("Calling Claude API for idea %d...", idea.ID)
	enriched, err := s.claudeService.EnrichIdea(ctx, input.RawText, username)
	if err != nil {
		log.Printf("ERROR: failed to enrich idea %d: %v", idea.ID, err)
		// Return the idea without enrichment - we'll try again later or manually
		return idea, nil, nil
	}
	log.Printf("Claude API returned successfully for idea %d", idea.ID)

	// Update the idea with enriched data
	if err := s.repo.UpdateEnriched(idea.ID, enriched); err != nil {
		log.Printf("Warning: failed to save enriched data for idea %d: %v", idea.ID, err)
	}

	// Refresh the idea from DB
	idea, _ = s.repo.GetByID(idea.ID)

	return idea, enriched, nil
}

// GetByID retrieves an idea by ID
func (s *IdeaService) GetByID(id int64) (*model.Idea, error) {
	return s.repo.GetByID(id)
}

// List retrieves ideas with optional filters
func (s *IdeaService) List(filter model.IdeaFilter) ([]*model.Idea, error) {
	return s.repo.List(filter)
}

// UpdateStatus updates the status of an idea
func (s *IdeaService) UpdateStatus(id int64, status model.IdeaStatus) error {
	return s.repo.UpdateStatus(id, status)
}

// UpdateAdminNotes updates the admin notes for an idea
func (s *IdeaService) UpdateAdminNotes(id int64, notes string) error {
	return s.repo.UpdateAdminNotes(id, notes)
}

// Delete removes an idea
func (s *IdeaService) Delete(id int64) error {
	return s.repo.Delete(id)
}

// Count returns the total number of ideas
func (s *IdeaService) Count(filter model.IdeaFilter) (int, error) {
	return s.repo.Count(filter)
}

// RateLimiter handles rate limiting per user and globally
type RateLimiter struct {
	userLimits  map[int64]*rate.Limiter
	globalLimit *rate.Limiter
	mu          sync.RWMutex
	perUser     int
}

func NewRateLimiter(perUser, global int) *RateLimiter {
	return &RateLimiter{
		userLimits:  make(map[int64]*rate.Limiter),
		globalLimit: rate.NewLimiter(rate.Every(time.Hour/time.Duration(global)), global),
		perUser:     perUser,
	}
}

func (rl *RateLimiter) Allow(userID int64) bool {
	// Check global limit first
	if !rl.globalLimit.Allow() {
		return false
	}

	// Check per-user limit
	rl.mu.Lock()
	limiter, exists := rl.userLimits[userID]
	if !exists {
		// Create new limiter for this user
		limiter = rate.NewLimiter(rate.Every(time.Hour/time.Duration(rl.perUser)), rl.perUser)
		rl.userLimits[userID] = limiter
	}
	rl.mu.Unlock()

	return limiter.Allow()
}

// Cleanup removes old user limiters (call periodically)
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	// Simple cleanup - just clear all (they'll be recreated on next request)
	rl.userLimits = make(map[int64]*rate.Limiter)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
