package protocol

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/PlotAI/plot-launcher/internal/repository"
)

// page is the browser view of an open request. A request made by a navigation
// renders it, so a click that fails says why in the browser instead of
// returning JSON to a tab the user is looking at.
type page struct {
	Title    string
	Message  string
	Remedy   string
	Command  string
	Location string
	Notices  []repository.Notice
	Code     string
}

// openPageTemplate is the single page the Protocol Handler serves. It carries
// no external assets so it renders while offline.
var openPageTemplate = template.Must(template.New("open").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
	:root { color-scheme: light dark; }
	body { margin: 0; padding: 4rem 1.5rem; font: 15px/1.55 system-ui, -apple-system, "Segoe UI", sans-serif; display: flex; justify-content: center; }
	main { width: 100%; max-width: 38rem; }
	h1 { font-size: 1.2rem; line-height: 1.35; margin: 0 0 1rem; }
	h2 { font-size: .75rem; text-transform: uppercase; letter-spacing: .06em; margin: 2rem 0 .5rem; opacity: .6; }
	p { margin: 0 0 .9rem; overflow-wrap: anywhere; }
	pre { margin: 0; padding: .7rem .9rem; border-radius: .5rem; background: rgba(127, 127, 127, .14); overflow-x: auto; }
	code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: .85rem; }
	.command { display: flex; align-items: flex-start; gap: .5rem; margin: 0 0 .9rem; }
	.command pre { flex: 1; }
	button { font: inherit; font-size: .8rem; padding: .3rem .75rem; border: 1px solid rgba(127, 127, 127, .5); border-radius: .4rem; background: transparent; color: inherit; cursor: pointer; }
	button:hover { background: rgba(127, 127, 127, .14); }
	.meta { margin-top: 2.5rem; font-size: .75rem; opacity: .55; }
</style>
</head>
<body>
<main>
	<h1>{{.Title}}</h1>
	{{with .Message}}<p>{{.}}</p>{{end}}
	{{with .Remedy}}<p>{{.}}</p>{{end}}
	{{with .Command}}{{template "command" .}}{{end}}
	{{with .Location}}<h2>Working directory</h2><pre><code>{{.}}</code></pre>{{end}}
	{{range .Notices}}
	<h2>Note</h2>
	<p>{{.Message}}</p>
	{{with .Command}}{{template "command" .}}{{end}}
	{{end}}
	{{with .Code}}<p class="meta">Plot Launcher reported {{.}}.</p>{{end}}
</main>
<script>
	for (const button of document.querySelectorAll(".command button")) {
		button.addEventListener("click", async () => {
			await navigator.clipboard.writeText(button.parentElement.querySelector("code").textContent);
			button.textContent = "Copied";
		});
	}
</script>
</body>
</html>
{{define "command"}}<div class="command"><pre><code>{{.}}</code></pre><button type="button">Copy</button></div>{{end}}
`))

// prefersPage reports whether the request is a browser navigation, which
// renders the result as a page rather than as JSON for a client.
func prefersPage(request *http.Request) bool {
	return strings.Contains(request.Header.Get("Accept"), "text/html")
}

func writePage(writer http.ResponseWriter, status int, view page) error {
	var rendered bytes.Buffer
	if err := openPageTemplate.Execute(&rendered, view); err != nil {
		return err
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(status)
	writer.Write(rendered.Bytes())
	return nil
}

// writeOpenFailure reports a failed open request, as a page for a browser
// navigation and as JSON for a client.
func writeOpenFailure(writer http.ResponseWriter, request *http.Request, pullRequest repository.PullRequest, err error) {
	failure := ErrorResponse{Error: err.Error()}
	view := page{
		Title:   "Could not open " + describe(pullRequest),
		Message: failure.Error,
	}
	var checkoutError *repository.CheckoutError
	if errors.As(err, &checkoutError) {
		failure.Code = checkoutError.Code
		failure.Remedy = checkoutError.Remedy
		failure.Command = checkoutError.Command
		view = page{
			Title:   "Could not open " + describe(pullRequest),
			Message: checkoutError.Message,
			Remedy:  checkoutError.Remedy,
			Command: checkoutError.Command,
			Code:    checkoutError.Code,
		}
	}

	if prefersPage(request) {
		if writePage(writer, http.StatusInternalServerError, view) == nil {
			return
		}
	}
	writeJSON(writer, http.StatusInternalServerError, failure)
}

// writeOpenSuccess reports a completed open request, as a page for a browser
// navigation and as JSON for a client.
func writeOpenSuccess(writer http.ResponseWriter, request *http.Request, pullRequest repository.PullRequest, checkout repository.Checkout) {
	if prefersPage(request) {
		view := page{
			Title:    "Opened " + describe(pullRequest) + " in your editor",
			Location: checkout.WorkingDirectory,
			Notices:  checkout.Notices,
		}
		if writePage(writer, http.StatusOK, view) == nil {
			return
		}
	}
	writeJSON(writer, http.StatusOK, NewOpenResult(pullRequest, checkout))
}

func describe(pullRequest repository.PullRequest) string {
	return pullRequest.Repository() + "#" + strconv.Itoa(pullRequest.Number)
}
