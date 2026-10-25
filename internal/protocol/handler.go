package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/PlotAI/plot-launcher/internal/repository"
)

// Opener receives open requests and returns the prepared working directory.
type Opener interface {
	OpenInEditor(ctx context.Context, pullRequest repository.PullRequest) (string, error)
}

// OpenResult is the response to a completed open request.
type OpenResult struct {
	Repository       string `json:"repository"`
	PullRequest      int    `json:"pull_request"`
	WorkingDirectory string `json:"working_directory"`
}

// NewOpenResult returns the response for a completed open request.
func NewOpenResult(pullRequest repository.PullRequest, workingDirectory string) OpenResult {
	return OpenResult{
		Repository:       pullRequest.Repository(),
		PullRequest:      pullRequest.Number,
		WorkingDirectory: workingDirectory,
	}
}

// Handler is the Protocol Handler: it receives browser requests for the
// launcher and forwards open requests to the Plot Launcher.
type Handler struct {
	opener Opener
	logger *slog.Logger
}

// NewHandler returns a Protocol Handler.
func NewHandler(opener Opener, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{opener: opener, logger: logger}
}

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/":
		h.receiveBrowserRequest(writer, request)
	case "/open":
		h.openInEditor(writer, request)
	default:
		writeError(writer, http.StatusNotFound, errors.New("not found"))
	}
}

func (h *Handler) receiveBrowserRequest(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		methodNotAllowed(writer, http.MethodGet, http.MethodHead)
		return
	}
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	if request.Method == http.MethodHead {
		return
	}
	writer.Write([]byte("Plot Launcher protocol handler\n" +
		"Send a request to /open?url=<pull request URL>\n" +
		"or to /open?repository=<owner/name>&pull_request=<number>.\n"))
}

func (h *Handler) openInEditor(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}

	openRequest, err := ParseQuery(request.URL.Query())
	if err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	pullRequest, err := openRequest.PullRequest()
	if err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}

	h.logger.Info("opening pull request in editor",
		"repository", pullRequest.Repository(),
		"pull_request", pullRequest.Number,
		"remote_address", request.RemoteAddr,
	)

	workingDirectory, err := h.opener.OpenInEditor(request.Context(), pullRequest)
	if err != nil {
		h.logger.Error("opening pull request in editor failed",
			"repository", pullRequest.Repository(),
			"pull_request", pullRequest.Number,
			"error", err,
		)
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	writeJSON(writer, http.StatusOK, NewOpenResult(pullRequest, workingDirectory))
}

func methodNotAllowed(writer http.ResponseWriter, methods ...string) {
	writer.Header().Set("Allow", strings.Join(methods, ", "))
	writeError(writer, http.StatusMethodNotAllowed, errors.New("method not allowed"))
}

func writeError(writer http.ResponseWriter, status int, err error) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	json.NewEncoder(writer).Encode(struct {
		Error string `json:"error"`
	}{Error: err.Error()})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	json.NewEncoder(writer).Encode(value)
}
