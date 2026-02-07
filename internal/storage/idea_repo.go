package storage

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/josinSbazin/idea-bot/internal/domain/model"
)

type IdeaRepository struct {
	db *sql.DB
}

func NewIdeaRepository() *IdeaRepository {
	return &IdeaRepository{db: DB()}
}

// Create inserts a new idea
func (r *IdeaRepository) Create(input model.CreateIdeaInput) (*model.Idea, error) {
	query := `
		INSERT INTO ideas (
			telegram_message_id, telegram_chat_id, telegram_user_id,
			telegram_username, telegram_first_name, raw_text, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.Exec(query,
		input.TelegramMessageID,
		input.TelegramChatID,
		input.TelegramUserID,
		input.TelegramUsername,
		input.TelegramFirstName,
		input.RawText,
		model.StatusNew,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

// GetByID retrieves an idea by ID
func (r *IdeaRepository) GetByID(id int64) (*model.Idea, error) {
	query := `
		SELECT id, telegram_message_id, telegram_chat_id, telegram_user_id,
			telegram_username, telegram_first_name, raw_text, enriched_json,
			title, category, priority, complexity, affected_repos, status,
			admin_notes, created_at, updated_at
		FROM ideas WHERE id = ?
	`

	idea := &model.Idea{}
	var affectedReposStr string

	err := r.db.QueryRow(query, id).Scan(
		&idea.ID,
		&idea.TelegramMessageID,
		&idea.TelegramChatID,
		&idea.TelegramUserID,
		&idea.TelegramUsername,
		&idea.TelegramFirstName,
		&idea.RawText,
		&idea.EnrichedJSON,
		&idea.Title,
		&idea.Category,
		&idea.Priority,
		&idea.Complexity,
		&affectedReposStr,
		&idea.Status,
		&idea.AdminNotes,
		&idea.CreatedAt,
		&idea.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse affected repos from JSON
	if affectedReposStr != "" {
		_ = json.Unmarshal([]byte(affectedReposStr), &idea.AffectedComponents)
	}

	// Parse enriched data
	_ = idea.ParseEnriched()

	return idea, nil
}

// List retrieves ideas with optional filters
func (r *IdeaRepository) List(filter model.IdeaFilter) ([]*model.Idea, error) {
	query := `
		SELECT id, telegram_message_id, telegram_chat_id, telegram_user_id,
			telegram_username, telegram_first_name, raw_text, enriched_json,
			title, category, priority, complexity, affected_repos, status,
			admin_notes, created_at, updated_at
		FROM ideas
	`

	var conditions []string
	var args []interface{}

	if len(filter.Status) > 0 {
		placeholders := make([]string, len(filter.Status))
		for i, s := range filter.Status {
			placeholders[i] = "?"
			args = append(args, string(s))
		}
		conditions = append(conditions, "status IN ("+strings.Join(placeholders, ",")+")")
	}

	if len(filter.Category) > 0 {
		placeholders := make([]string, len(filter.Category))
		for i, c := range filter.Category {
			placeholders[i] = "?"
			args = append(args, string(c))
		}
		conditions = append(conditions, "category IN ("+strings.Join(placeholders, ",")+")")
	}

	if len(filter.Priority) > 0 {
		placeholders := make([]string, len(filter.Priority))
		for i, p := range filter.Priority {
			placeholders[i] = "?"
			args = append(args, string(p))
		}
		conditions = append(conditions, "priority IN ("+strings.Join(placeholders, ",")+")")
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ideas []*model.Idea
	for rows.Next() {
		idea := &model.Idea{}
		var affectedReposStr string

		err := rows.Scan(
			&idea.ID,
			&idea.TelegramMessageID,
			&idea.TelegramChatID,
			&idea.TelegramUserID,
			&idea.TelegramUsername,
			&idea.TelegramFirstName,
			&idea.RawText,
			&idea.EnrichedJSON,
			&idea.Title,
			&idea.Category,
			&idea.Priority,
			&idea.Complexity,
			&affectedReposStr,
			&idea.Status,
			&idea.AdminNotes,
			&idea.CreatedAt,
			&idea.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if affectedReposStr != "" {
			_ = json.Unmarshal([]byte(affectedReposStr), &idea.AffectedComponents)
		}
		_ = idea.ParseEnriched()

		ideas = append(ideas, idea)
	}

	return ideas, rows.Err()
}

// UpdateEnriched updates the enriched data for an idea
func (r *IdeaRepository) UpdateEnriched(id int64, enriched *model.EnrichedIdea) error {
	enrichedJSON, err := json.Marshal(enriched)
	if err != nil {
		return err
	}

	affectedReposJSON, err := json.Marshal(enriched.AffectedComponents)
	if err != nil {
		return err
	}

	query := `
		UPDATE ideas SET
			enriched_json = ?,
			title = ?,
			category = ?,
			priority = ?,
			complexity = ?,
			affected_repos = ?,
			updated_at = ?
		WHERE id = ?
	`

	_, err = r.db.Exec(query,
		string(enrichedJSON),
		enriched.Title,
		enriched.Category,
		enriched.Priority,
		enriched.Complexity,
		string(affectedReposJSON),
		time.Now(),
		id,
	)
	return err
}

// UpdateStatus updates the status of an idea
func (r *IdeaRepository) UpdateStatus(id int64, status model.IdeaStatus) error {
	query := `UPDATE ideas SET status = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, string(status), time.Now(), id)
	return err
}

// UpdateAdminNotes updates the admin notes for an idea
func (r *IdeaRepository) UpdateAdminNotes(id int64, notes string) error {
	query := `UPDATE ideas SET admin_notes = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, notes, time.Now(), id)
	return err
}

// Count returns the total number of ideas matching the filter
func (r *IdeaRepository) Count(filter model.IdeaFilter) (int, error) {
	query := `SELECT COUNT(*) FROM ideas`

	var conditions []string
	var args []interface{}

	if len(filter.Status) > 0 {
		placeholders := make([]string, len(filter.Status))
		for i, s := range filter.Status {
			placeholders[i] = "?"
			args = append(args, string(s))
		}
		conditions = append(conditions, "status IN ("+strings.Join(placeholders, ",")+")")
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	var count int
	err := r.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// Delete removes an idea by ID
func (r *IdeaRepository) Delete(id int64) error {
	query := `DELETE FROM ideas WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}

// ListSummaries returns lightweight list of ideas for duplicate checking
func (r *IdeaRepository) ListSummaries() ([]model.IdeaSummary, error) {
	query := `SELECT id, title, raw_text FROM ideas WHERE status NOT IN ('rejected', 'implemented') ORDER BY created_at DESC LIMIT 100`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []model.IdeaSummary
	for rows.Next() {
		var s model.IdeaSummary
		if err := rows.Scan(&s.ID, &s.Title, &s.RawText); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}

	return summaries, rows.Err()
}
