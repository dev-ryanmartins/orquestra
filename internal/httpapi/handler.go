package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dev-ryanmartins/orquestra/internal/service"
	"github.com/dev-ryanmartins/orquestra/internal/store"
)

const maxRequestBody = 1 << 20

type Handler struct {
	service  *service.Service
	logger   *slog.Logger
	basePath string
	mux      *http.ServeMux
}

type createTaskRequest struct {
	Name    string          `json:"name"`
	Payload json.RawMessage `json:"payload"`
}

type statsResponse struct {
	store.Snapshot
	QueueDepth    int `json:"queue_depth"`
	QueueCapacity int `json:"queue_capacity"`
}

func New(taskService *service.Service, logger *slog.Logger, basePath string) http.Handler {
	handler := &Handler{
		service:  taskService,
		logger:   logger,
		basePath: strings.TrimSuffix(basePath, "/"),
		mux:      http.NewServeMux(),
	}
	handler.registerRoutes()
	return handler
}

func (h *Handler) registerRoutes() {
	h.mux.HandleFunc("GET /healthz", h.health)
	h.mux.HandleFunc("GET /stats", h.stats)
	h.mux.HandleFunc("POST /tasks", h.createTask)
	h.mux.HandleFunc("GET /tasks/{id}", h.getTask)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	trimmed := r.URL.Path
	for _, prefix := range []string{h.basePath, "/api"} {
		if prefix != "" && (trimmed == prefix || strings.HasPrefix(trimmed, prefix+"/")) {
			trimmed = strings.TrimPrefix(trimmed, prefix)
			if trimmed == "" {
				trimmed = "/"
			}
			break
		}
	}

	request := r.Clone(r.Context())
	request.URL.Path = trimmed
	h.mux.ServeHTTP(w, request)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) stats(w http.ResponseWriter, _ *http.Request) {
	snapshot := h.service.Stats()
	writeJSON(w, http.StatusOK, statsResponse{
		Snapshot:      snapshot,
		QueueDepth:    h.service.QueueDepth(),
		QueueCapacity: h.service.QueueCapacity(),
	})
}

func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	var input createTaskRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "corpo JSON inválido")
		return
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "o corpo deve conter apenas um objeto JSON")
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "o campo name é obrigatório")
		return
	}
	if len(input.Name) > 120 {
		writeError(w, http.StatusBadRequest, "o campo name deve ter no máximo 120 caracteres")
		return
	}
	if len(input.Payload) == 0 {
		input.Payload = json.RawMessage(`{}`)
	}
	if !json.Valid(input.Payload) {
		writeError(w, http.StatusBadRequest, "o campo payload deve ser um JSON válido")
		return
	}

	value, err := h.service.Submit(input.Name, input.Payload)
	if err != nil {
		if errors.Is(err, service.ErrQueueFull) {
			writeError(w, http.StatusServiceUnavailable, "a fila está cheia; tente novamente em instantes")
			return
		}
		h.logger.Error("failed to submit task", "error", err)
		writeError(w, http.StatusInternalServerError, "não foi possível enfileirar a tarefa")
		return
	}

	writeJSON(w, http.StatusAccepted, value)
}

func (h *Handler) getTask(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.Get(r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "tarefa não encontrada")
		return
	}
	if err != nil {
		h.logger.Error("failed to read task", "error", err)
		writeError(w, http.StatusInternalServerError, "não foi possível consultar a tarefa")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("extra JSON value")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
