package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PlotAI/plot-launcher/internal/repository"
)

type fakeOpener struct {
	pullRequest      repository.PullRequest
	workingDirectory string
	err              error
	calls            int
}

func (f *fakeOpener) OpenInEditor(_ context.Context, pullRequest repository.PullRequest) (string, error) {
	f.calls++
	f.pullRequest = pullRequest
	return f.workingDirectory, f.err
}

func newTestHandler(opener Opener) *Handler {
	return NewHandler(opener, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestHandlerOpenInEditor(t *testing.T) {
	opener := &fakeOpener{workingDirectory: "/workspace/name#pr7"}
	handler := newTestHandler(opener)

	request := httptest.NewRequest(http.MethodGet, "/open?repository=owner/name&pull_request=7", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if opener.calls != 1 {
		t.Fatalf("opener calls = %d, want 1", opener.calls)
	}
	if opener.pullRequest.Repository() != "owner/name" || opener.pullRequest.Number != 7 {
		t.Fatalf("opener pull request = %+v", opener.pullRequest)
	}

	var result OpenResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := NewOpenResult(opener.pullRequest, opener.workingDirectory)
	if result != want {
		t.Fatalf("result = %+v, want %+v", result, want)
	}
}

func TestHandlerRejectsInvalidRequest(t *testing.T) {
	opener := &fakeOpener{}
	handler := newTestHandler(opener)

	for _, target := range []string{
		"/open",
		"/open?repository=owner/name",
		"/open?repository=owner/name&pull_request=0",
		"/open?repository=owner/name&pull_request=abc",
	} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want %d", target, recorder.Code, http.StatusBadRequest)
		}
	}
	if opener.calls != 0 {
		t.Fatalf("opener calls = %d, want 0", opener.calls)
	}
}

func TestHandlerReportsOpenError(t *testing.T) {
	opener := &fakeOpener{err: errors.New("checkout failed")}
	handler := newTestHandler(opener)

	request := httptest.NewRequest(http.MethodGet, "/open?repository=owner/name&pull_request=7", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(recorder.Body.String(), "checkout failed") {
		t.Fatalf("response = %q, want the error", recorder.Body.String())
	}
}

func TestHandlerRejectsUnsupportedMethods(t *testing.T) {
	handler := newTestHandler(&fakeOpener{})
	request := httptest.NewRequest(http.MethodPost, "/open?repository=owner/name&pull_request=7", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	if recorder.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("Allow = %q, want %q", recorder.Header().Get("Allow"), http.MethodGet)
	}
}

func TestHandlerRootDescribesInterface(t *testing.T) {
	handler := newTestHandler(&fakeOpener{})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "/open?url=") {
		t.Fatalf("root response = %q, want the /open route", recorder.Body.String())
	}
}

func TestHandlerUnknownPath(t *testing.T) {
	handler := newTestHandler(&fakeOpener{})
	request := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestHandlerOpenInEditorAcceptsPullRequestURL(t *testing.T) {
	opener := &fakeOpener{workingDirectory: "/workspace/name#pr7"}
	handler := newTestHandler(opener)

	request := httptest.NewRequest(http.MethodGet, "/open?url=https%3A%2F%2Fgithub.com%2FPlotCustomer%2Fmenu%2Fpull%2F6", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if opener.calls != 1 {
		t.Fatalf("opener calls = %d, want 1", opener.calls)
	}
	if opener.pullRequest.Repository() != "PlotCustomer/menu" || opener.pullRequest.Number != 6 {
		t.Fatalf("opener pull request = %+v", opener.pullRequest)
	}
}
