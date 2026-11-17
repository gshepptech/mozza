package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gshepptech/mozza/internal/compile"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
	"github.com/gshepptech/mozza/internal/targets"
)

// targetsListResponse is the body of GET /api/v1/targets.
type targetsListResponse struct {
	Default string           `json:"default"`
	Targets []targets.Target `json:"targets"`
}

// compileRequest is the body of POST /api/v1/compile. An empty Target means
// the caller does not care and gets targets.Default.
type compileRequest struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// compileFileResponse is one generated artifact.
type compileFileResponse struct {
	Path string `json:"path"`
	// Content is the file body as text. compile.OutputFile holds bytes, which
	// would marshal to base64, and clients want readable YAML.
	Content string `json:"content"`
}

// compileResponse is the body of POST /api/v1/compile.
type compileResponse struct {
	Target   string                `json:"target"`
	Files    []compileFileResponse `json:"files"`
	Summary  string                `json:"summary"`
	Warnings []string              `json:"warnings"`
}

// handleTargets lists the compile targets this server can compile to.
func (s *Server) handleTargets() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		JSON(w, http.StatusOK, targetsListResponse{
			Targets: targets.List(s.cfg.Targets),
			Default: targets.Default,
		})
	}
}

// handleCompile compiles a recipe to the named target and returns the generated
// files. It applies nothing: no store writes, no cluster contact. That is what
// makes it safe for any authenticated user to call.
func (s *Server) handleCompile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req compileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if strings.TrimSpace(req.Source) == "" {
			Error(w, http.StatusBadRequest, "source is required")
			return
		}

		target := req.Target
		if target == "" {
			target = targets.Default
		}

		// Checked before parsing so a typo'd target is not masked by a recipe error.
		compiler, err := s.cfg.Targets.Get(target)
		if err != nil {
			Error(w, http.StatusBadRequest, fmt.Sprintf(
				"unknown target %q; valid targets: %s",
				target, strings.Join(s.cfg.Targets.Targets(), ", ")))
			return
		}

		p, msg := s.planFromSource(req.Source)
		if msg != "" {
			Error(w, http.StatusBadRequest, msg)
			return
		}

		result, err := compiler.Compile(r.Context(), p)
		if err != nil {
			Error(w, http.StatusInternalServerError, fmt.Sprintf("compile failed: %v", err))
			return
		}

		JSON(w, http.StatusOK, toCompileResponse(target, result))
	}
}

// planFromSource parses, builds, and validates a plan from recipe source. It
// returns a client-facing message on failure rather than an error, because
// every failure here is the caller's input being wrong, not the server's fault.
func (s *Server) planFromSource(source string) (*plan.AppPlan, string) {
	rec, err := recipe.NewParser(source).Parse()
	if err != nil {
		return nil, fmt.Sprintf("recipe parse failed: %v", err)
	}

	p, err := plan.Build(rec)
	if err != nil {
		return nil, fmt.Sprintf("plan build failed: %v", err)
	}

	if err := plan.Validate(p); err != nil {
		return nil, fmt.Sprintf("plan validation failed: %v", err)
	}

	return p, ""
}

// toCompileResponse renders a compile result as JSON. Warnings are always a
// list, never null: a target that silently drops a feature reports it here, so
// clients must be able to range over the field unconditionally.
func toCompileResponse(target string, result *compile.Result) compileResponse {
	resp := compileResponse{
		Target:   target,
		Summary:  result.Summary,
		Files:    make([]compileFileResponse, len(result.Files)),
		Warnings: []string{},
	}

	for i, f := range result.Files {
		resp.Files[i] = compileFileResponse{Path: f.Path, Content: string(f.Content)}
	}
	if len(result.Warnings) > 0 {
		resp.Warnings = result.Warnings
	}

	return resp
}
