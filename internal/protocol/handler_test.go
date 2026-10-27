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
	pullRequest repository.PullRequest
	checkout    repository.Checkout
	err         error
	calls       int
}

func (f *fakeOpener) OpenInEditor(_ context.Context, pullRequest repository.PullRequest) (repository.Checkout, error) {
	f.calls++
	f.pullRequest = pullRequest
	return f.checkout, f.err
}

func newTestHandler(opener Opener) *Handler {
	return NewHandler(opener, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestHandlerOpenInEditor(t *testing.T) {
	opener := &fakeOpener{checkout: repository.Checkout{
		WorkingDirectory: "/workspace/name",
		Notices:          []repository.Notice{{Code: repository.NoticeStashKept, Message: "changes stay in the stash"}},
	}}
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
	want := NewOpenResult(opener.pullRequest, opener.checkout)
	if result.Repository != want.Repository || result.PullRequest != want.PullRequest ||
		result.WorkingDirectory != want.WorkingDirectory || len(result.Notices) != 1 ||
		result.Notices[0].Code != repository.NoticeStashKept {
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

func TestHandlerReportsCheckoutFailureDetails(t *testing.T) {
	opener := &fakeOpener{err: &repository.CheckoutError{
		Code:    repository.CodeRemoteMismatch,
		Message: "working directory tracks another repository",
		Remedy:  "Remove the checkout and open the pull request again.",
		Command: "rm -rf /workspace/name",
	}}
	handler := newTestHandler(opener)

	request := httptest.NewRequest(http.MethodGet, "/open?repository=owner/name&pull_request=7", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	var failure ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &failure); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if failure.Code != repository.CodeRemoteMismatch {
		t.Fatalf("code = %q, want %q", failure.Code, repository.CodeRemoteMismatch)
	}
	if !strings.Contains(failure.Error, "tracks another repository") {
		t.Fatalf("error = %q, want the failure message", failure.Error)
	}
	if !strings.Contains(failure.Remedy, "Remove the checkout") {
		t.Fatalf("remedy = %q, want what to do", failure.Remedy)
	}
	if failure.Command != "rm -rf /workspace/name" {
		t.Fatalf("command = %q, want the repair command", failure.Command)
	}
}

func TestHandlerServesPageToBrowser(t *testing.T) {
	opener := &fakeOpener{checkout: repository.Checkout{
		WorkingDirectory: "/workspace/name",
		Notices: []repository.Notice{{
			Code:    repository.NoticeStashKept,
			Message: "the uncommitted changes stay in the stash entry",
			Command: "git -C /workspace/name stash pop",
		}},
	}}
	handler := newTestHandler(opener)

	request := httptest.NewRequest(http.MethodGet, "/open?repository=owner/name&pull_request=7", nil)
	request.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", contentType)
	}
	body := recorder.Body.String()
	for _, want := range []string{
		"Opened owner/name#7 in your editor",
		"/workspace/name",
		"the uncommitted changes stay in the stash entry",
		"git -C /workspace/name stash pop",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page does not contain %q:\n%s", want, body)
		}
	}
}

func TestHandlerServesFailurePageToBrowser(t *testing.T) {
	opener := &fakeOpener{err: &repository.CheckoutError{
		Code:    repository.CodeRemoteMismatch,
		Message: "working directory /workspace/name tracks another repository",
		Remedy:  "Remove the checkout and open the pull request again.",
		Command: "rm -rf /workspace/name",
	}}
	handler := newTestHandler(opener)

	request := httptest.NewRequest(http.MethodGet, "/open?repository=owner/name&pull_request=7", nil)
	request.Header.Set("Accept", "text/html")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	body := recorder.Body.String()
	for _, want := range []string{
		"Could not open owner/name#7",
		"tracks another repository",
		"Remove the checkout and open the pull request again.",
		"rm -rf /workspace/name",
		repository.CodeRemoteMismatch,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page does not contain %q:\n%s", want, body)
		}
	}
}

func TestHandlerEscapesPageContent(t *testing.T) {
	opener := &fakeOpener{err: &repository.CheckoutError{
		Message: "<script>alert(1)</script>",
	}}
	handler := newTestHandler(opener)

	request := httptest.NewRequest(http.MethodGet, "/open?repository=owner/name&pull_request=7", nil)
	request.Header.Set("Accept", "text/html")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	body := recorder.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatalf("page does not escape the message:\n%s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("page does not escape the message:\n%s", body)
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
	opener := &fakeOpener{checkout: repository.Checkout{WorkingDirectory: "/workspace/name"}}
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
