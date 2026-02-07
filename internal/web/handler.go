package web

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/josinSbazin/idea-bot/internal/config"
	"github.com/josinSbazin/idea-bot/internal/domain/model"
	"github.com/josinSbazin/idea-bot/internal/domain/service"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Handler struct {
	ideaService *service.IdeaService
	templateMap map[string]*template.Template
}

func NewHandler(ideaService *service.IdeaService) (*Handler, error) {
	funcMap := template.FuncMap{
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "..."
		},
		"formatDate": func(t time.Time) string {
			return t.Format("02.01.2006 15:04")
		},
		"statusLabel": func(s model.IdeaStatus) string {
			return s.Label()
		},
		"categoryLabel": func(c model.IdeaCategory) string {
			return c.Label()
		},
		"priorityLabel": func(p model.IdeaPriority) string {
			return p.Label()
		},
		"complexityLabel": func(c model.IdeaComplexity) string {
			return c.Label()
		},
		"join": func(arr []string, sep string) string {
			return strings.Join(arr, sep)
		},
	}

	// Parse each page template separately with layout
	templates := make(map[string]*template.Template)
	pages := []string{"ideas.html", "idea.html"}

	for _, page := range pages {
		tmpl, err := template.New("").Funcs(funcMap).ParseFS(templatesFS, "templates/layout.html", "templates/"+page)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", page, err)
		}
		templates[page] = tmpl
	}

	return &Handler{
		ideaService:  ideaService,
		templateMap:  templates,
	}, nil
}

// SetupRoutes configures HTTP routes
func (h *Handler) SetupRoutes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", h.handleIndex)
	mux.HandleFunc("/ideas", h.handleIdeas)
	mux.HandleFunc("/ideas/", h.handleIdeaDetail)
	mux.HandleFunc("/health", h.handleHealth)

	// Apply middleware
	cfg := config.Get()
	var handler http.Handler = mux
	handler = Recover(handler)
	handler = Logging(handler)
	handler = BasicAuth(cfg.Web.Username, cfg.Web.Password)(handler)

	return handler
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/ideas", http.StatusFound)
}

func (h *Handler) handleIdeas(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.handleIdeasPost(w, r)
		return
	}

	// Parse filters from query params
	filter := model.IdeaFilter{
		Limit: 100,
	}

	if status := r.URL.Query().Get("status"); status != "" {
		for _, s := range strings.Split(status, ",") {
			filter.Status = append(filter.Status, model.IdeaStatus(s))
		}
	}

	if category := r.URL.Query().Get("category"); category != "" {
		for _, c := range strings.Split(category, ",") {
			filter.Category = append(filter.Category, model.IdeaCategory(c))
		}
	}

	if priority := r.URL.Query().Get("priority"); priority != "" {
		for _, p := range strings.Split(priority, ",") {
			filter.Priority = append(filter.Priority, model.IdeaPriority(p))
		}
	}

	ideas, err := h.ideaService.List(filter)
	if err != nil {
		log.Printf("Error listing ideas: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get counts for stats
	totalCount, _ := h.ideaService.Count(model.IdeaFilter{})
	newCount, _ := h.ideaService.Count(model.IdeaFilter{Status: []model.IdeaStatus{model.StatusNew}})

	data := map[string]interface{}{
		"Title":         "Список идей",
		"Ideas":         ideas,
		"TotalCount":    totalCount,
		"NewCount":      newCount,
		"AllStatuses":   model.AllStatuses(),
		"AllCategories": model.AllCategories(),
		"AllPriorities": model.AllPriorities(),
		"Filter":        filter,
	}

	h.render(w, "ideas.html", data)
}

func (h *Handler) handleIdeasPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	action := r.FormValue("action")
	idStr := r.FormValue("id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	switch action {
	case "update_status":
		status := model.IdeaStatus(r.FormValue("status"))
		if err := h.ideaService.UpdateStatus(id, status); err != nil {
			log.Printf("Error updating status: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	case "update_notes":
		notes := r.FormValue("notes")
		if err := h.ideaService.UpdateAdminNotes(id, notes); err != nil {
			log.Printf("Error updating notes: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	case "delete":
		if err := h.ideaService.Delete(id); err != nil {
			log.Printf("Error deleting idea: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/ideas", http.StatusFound)
		return
	}

	// Redirect back to the idea detail page
	http.Redirect(w, r, fmt.Sprintf("/ideas/%d", id), http.StatusFound)
}

func (h *Handler) handleIdeaDetail(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path /ideas/{id}
	path := strings.TrimPrefix(r.URL.Path, "/ideas/")
	if path == "" {
		http.Redirect(w, r, "/ideas", http.StatusFound)
		return
	}

	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	idea, err := h.ideaService.GetByID(id)
	if err != nil {
		log.Printf("Error getting idea %d: %v", id, err)
		http.NotFound(w, r)
		return
	}

	data := map[string]interface{}{
		"Title":       fmt.Sprintf("Идея #%d", idea.ID),
		"Idea":        idea,
		"AllStatuses": model.AllStatuses(),
	}

	h.render(w, "idea.html", data)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, ok := h.templateMap[name]
	if !ok {
		log.Printf("Template not found: %s", name)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
