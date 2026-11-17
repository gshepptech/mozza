package plan_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
)

// minCompiledDocRecipes is the floor on how many documentation blocks must
// actually reach the end of the pipeline. A bare non-zero check would let
// someone silence the suite by marking every awkward block `ignore`, so removing
// doc examples has to be a deliberate act of lowering this number.
const minCompiledDocRecipes = 5

// appDeclaration matches the line that makes a fenced block a recipe rather
// than shell, YAML or prose.
var appDeclaration = regexp.MustCompile(`(?m)^\s*App:`)

// fencedBlock is one fenced code block lifted out of a markdown file, kept with
// its source position so a failure names a fence the reader can go and edit.
type fencedBlock struct {
	info string // the opening fence's info string, e.g. "mozza ignore"
	body string
	line int // 1-based line number of the opening fence
}

// TestDocRecipesCompileClean runs every recipe example in the documentation
// through the same Parse → Build → Validate pipeline loadPlan uses, so a doc
// example the grammar rejects fails the build rather than the reader's first
// deploy. A fenced block counts as a recipe when its body declares an App:.
// Blocks that are deliberately illustrative (fragments, grammar metasyntax, an
// HTTP request that happens to carry a recipe body) opt out with an `ignore`
// field on the fence itself, as in ```mozza ignore. The marker travels on the
// fence so it cannot drift away from the block it marks.
//
// The pipeline stops at Validate, which is where loadPlan stops too.
//
// The subtests run sequentially on purpose: they tally the counts the floor
// below is asserted against, which a parallel subtest would race and defer past
// the assertion.
func TestDocRecipesCompileClean(t *testing.T) {
	root := repoRoot(t)

	var compiled, skipped int

	for _, file := range markdownFiles(t, root) {
		src, err := os.ReadFile(file)
		require.NoError(t, err)

		rel, err := filepath.Rel(root, file)
		require.NoError(t, err)

		for _, block := range fencedBlocks(string(src)) {
			if !appDeclaration.MatchString(block.body) {
				continue
			}

			where := fmt.Sprintf("%s:%d", filepath.ToSlash(rel), block.line)

			t.Run(where, func(t *testing.T) {
				if slices.Contains(strings.Fields(block.info), "ignore") {
					skipped++
					t.Skipf("fence marked ignore: ```%s", block.info)
				}

				compileDocRecipe(t, where, block.body)
				compiled++
			})
		}
	}

	t.Logf("compiled=%d skipped=%d", compiled, skipped)
	require.GreaterOrEqual(t, compiled, minCompiledDocRecipes,
		"only %d documentation recipes compiled, expected at least %d — if an example was "+
			"legitimately removed, lower minCompiledDocRecipes; if a block is not meant to "+
			"compile, mark its fence ```<lang> ignore",
		compiled, minCompiledDocRecipes)
}

// TestExampleRecipesCompileClean runs every recipe under examples/ through the
// same pipeline, so a shipped example cannot drift out of the grammar.
func TestExampleRecipesCompileClean(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join(repoRoot(t), "examples", "*.mozza"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "no example recipes found")

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(file)
			require.NoError(t, err)
			compileDocRecipe(t, filepath.Base(file), string(src))
		})
	}
}

// compileDocRecipe runs one documentation block through the recipe pipeline.
func compileDocRecipe(t *testing.T, where, body string) {
	t.Helper()

	r, err := recipe.NewParser(body).Parse()
	require.NoError(t, err, "Parse failed for %s:\n%s", where, body)

	p, err := plan.Build(r)
	require.NoError(t, err, "Build failed for %s:\n%s", where, body)
	require.NoError(t, plan.Validate(p), "Validate failed for %s:\n%s", where, body)
}

// fencedBlocks returns every fenced code block in src along with its opening
// fence's info string. Leading whitespace is trimmed before matching, so an
// indented fence is scanned rather than silently passed over, and the body is
// dedented by the opening fence's own indentation. An indented recipe handed
// to the parser with its leading spaces intact would fail to parse and be
// reported as broken documentation when it is nothing of the kind.
func fencedBlocks(src string) []fencedBlock {
	var (
		blocks []fencedBlock
		open   *fencedBlock
		indent string
		body   []string
	)

	for i, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)

		if !strings.HasPrefix(trimmed, "```") {
			if open != nil {
				body = append(body, strings.TrimPrefix(line, indent))
			}

			continue
		}

		if open == nil {
			open = &fencedBlock{
				info: strings.TrimSpace(strings.TrimPrefix(trimmed, "```")),
				line: i + 1,
			}
			indent = line[:strings.Index(line, "```")]
			body = nil

			continue
		}

		open.body = strings.Join(body, "\n")
		blocks = append(blocks, *open)
		open = nil
	}

	return blocks
}

// markdownFiles returns README.md plus every markdown file under docs/.
func markdownFiles(t *testing.T, root string) []string {
	t.Helper()

	files := []string{filepath.Join(root, "README.md")}

	err := filepath.WalkDir(filepath.Join(root, "docs"),
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() && strings.HasSuffix(path, ".md") {
				files = append(files, path)
			}

			return nil
		})
	require.NoError(t, err)

	return files
}

// repoRoot walks up from the working directory to the module root. It fails the
// test rather than returning an empty path, so a root that cannot be found
// reports as itself instead of as a suite that quietly found no documentation.
func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repoRoot: no go.mod found above %s", dir)
		}

		dir = parent
	}
}
