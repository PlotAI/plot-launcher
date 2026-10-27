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

// Opener receives open requests and returns the prepared checkout.
type Opener interface {
	OpenInEditor(ctx context.Context, pullRequest repository.PullRequest) (repository.Checkout, error)
}

// OpenResult is the response to a completed open request.
type OpenResult struct {
	Repository       string              `json:"repository"`
	PullRequest      int                 `json:"pull_request"`
	WorkingDirectory string              `json:"working_directory"`
	Notices          []repository.Notice `json:"notices,omitempty"`
}

// NewOpenResult returns the response for a completed open request.
func NewOpenResult(pullRequest repository.PullRequest, checkout repository.Checkout) OpenResult {
	return OpenResult{
		Repository:       pullRequest.Repository(),
		PullRequest:      pullRequest.Number,
		WorkingDirectory: checkout.WorkingDirectory,
		Notices:          checkout.Notices,
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

	checkout, err := h.opener.OpenInEditor(request.Context(), pullRequest)
	if err != nil {
		h.logger.Error("opening pull request in editor failed",
			"repository", pullRequest.Repository(),
			"pull_request", pullRequest.Number,
			"error", err,
		)
		writeOpenFailure(writer, request, pullRequest, err)
		return
	}
	writeOpenSuccess(writer, request, pullRequest, checkout)
}

// ErrorResponse is the response to a failed request. Code, Remedy, and
// Command tell a client which failure it was, what to do about it, and the
// command that does it.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Remedy  string `json:"remedy,omitempty"`
	Command string `json:"command,omitempty"`
}

func methodNotAllowed(writer http.ResponseWriter, methods ...string) {
	writer.Header().Set("Allow", strings.Join(methods, ", "))
	writeError(writer, http.StatusMethodNotAllowed, errors.New("method not allowed"))
}

func writeError(writer http.ResponseWriter, status int, err error) {
	writeJSON(writer, status, ErrorResponse{Error: err.Error()})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	json.NewEncoder(writer).Encode(value)
}
