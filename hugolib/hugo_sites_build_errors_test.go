package hugolib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gohugoio/hugo/htesting"

	qt "github.com/frankban/quicktest"
	"github.com/gohugoio/hugo/common/herrors"
)

type testSiteBuildErrorAsserter struct {
	name string
	c    *qt.C
}

func (t testSiteBuildErrorAsserter) getFileError(err error) herrors.FileError {
	t.c.Assert(err, qt.Not(qt.IsNil), qt.Commentf(t.name))
	fe := herrors.UnwrapFileError(err)
	t.c.Assert(fe, qt.Not(qt.IsNil))
	return fe
}

func (t testSiteBuildErrorAsserter) assertLineNumber(lineNumber int, err error) {
	t.c.Helper()
	fe := t.getFileError(err)
	t.c.Assert(fe.Position().LineNumber, qt.Equals, lineNumber, qt.Commentf(err.Error()))
}

func (t testSiteBuildErrorAsserter) assertErrorMessage(e1, e2 string) {
	// The error message will contain filenames with OS slashes. Normalize before compare.
	e1, e2 = filepath.ToSlash(e1), filepath.ToSlash(e2)
	t.c.Assert(e2, qt.Contains, e1)
}

func TestSiteBuildErrors(t *testing.T) {
	const (
		yamlcontent = "yamlcontent"
		tomlcontent = "tomlcontent"
		jsoncontent = "jsoncontent"
		shortcode   = "shortcode"
		base        = "base"
		single      = "single"
	)

	// TODO(bep) add content tests after https://github.com/gohugoio/hugo/issues/5324
	// is implemented.

	tests := []struct {
		name              string
		fileType          string
		fileFixer         func(content string) string
		assertCreateError func(a testSiteBuildErrorAsserter, err error)
		assertBuildError  func(a testSiteBuildErrorAsserter, err error)
	}{

		{
			name:     "Base template parse failed",
			fileType: base,
			fileFixer: func(content string) string {
				return strings.Replace(content, ".Title }}", ".Title }", 1)
			},
			// Base templates gets parsed at build time.
			assertBuildError: func(a testSiteBuildErrorAsserter, err error) {
				a.assertLineNumber(4, err)
			},
		},
		{
			name:     "Base template execute failed",
			fileType: base,
			fileFixer: func(content string) string {
				return strings.Replace(content, ".Title", ".Titles", 1)
			},
			assertBuildError: func(a testSiteBuildErrorAsserter, err error) {
				a.assertLineNumber(4, err)
			},
		},
		{
			name:     "Single template parse failed",
			fileType: single,
			fileFixer: func(content string) string {
				return strings.Replace(content, ".Title }}", ".Title }", 1)
			},
			assertCreateError: func(a testSiteBuildErrorAsserter, err error) {
				fe := a.getFileError(err)
				a.c.Assert(fe.Position().LineNumber, qt.Equals, 5)
				a.c.Assert(fe.Position().ColumnNumber, qt.Equals, 1)
				a.assertErrorMessage("\"layouts/foo/single.html:5:1\": parse failed: template: foo/single.html:5: unexpected \"}\" in operand", fe.Error())
			},
		},
		{
			name:     "Single template execute failed",
			fileType: single,
			fileFixer: func(content string) string {
				return strings.Replace(content, ".Title", ".Titles", 1)
			},
			assertBuildError: func(a testSiteBuildErrorAsserter, err error) {
				fe := a.getFileError(err)
				a.c.Assert(fe.Position().LineNumber, qt.Equals, 5)
				a.c.Assert(fe.Position().ColumnNumber, qt.Equals, 14)
				a.assertErrorMessage("\"layouts/_default/single.html:5:14\": execute of template failed", fe.Error())
			},
		},
		{
			name:     "Single template execute failed, long keyword",
			fileType: single,
			fileFixer: func(content string) string {
				return strings.Replace(content, ".Title", ".ThisIsAVeryLongTitle", 1)
			},
			assertBuildError: func(a testSiteBuildErrorAsserter, err error) {
				fe := a.getFileError(err)
				a.c.Assert(fe.Position().LineNumber, qt.Equals, 5)
				a.c.Assert(fe.Position().ColumnNumber, qt.Equals, 14)
				a.assertErrorMessage("\"layouts/_default/single.html:5:14\": execute of template failed", fe.Error())
			},
		},
		{
			name:     "Shortcode parse failed",
			fileType: shortcode,
			fileFixer: func(content string) string {
				return strings.Replace(content, ".Title }}", ".Title }", 1)
			},
			assertCreateError: func(a testSiteBuildErrorAsserter, err error) {
				a.assertLineNumber(4, err)
			},
		},
		{
			name:     "Shortcode execute failed",
			fileType: shortcode,
			fileFixer: func(content string) string {
				return strings.Replace(content, ".Title", ".Titles", 1)
			},
			assertBuildError: func(a testSiteBuildErrorAsserter, err error) {
				fe := a.getFileError(err)
				// Make sure that it contains both the content file and template
				a.assertErrorMessage(`"content/myyaml.md:7:10": failed to render shortcode "sc": failed to process shortcode: "layouts/shortcodes/sc.html:4:22": execute of template failed: template: shortcodes/sc.html:4:22: executing "shortcodes/sc.html" at <.Page.Titles>: can't evaluate field Titles in type page.Page`, fe.Error())
				a.c.Assert(fe.Position().LineNumber, qt.Equals, 7)

			},
		},
		{
			name:     "Shortode does not exist",
			fileType: yamlcontent,
			fileFixer: func(content string) string {
				return strings.Replace(content, "{{< sc >}}", "{{< nono >}}", 1)
			},
			assertBuildError: func(a testSiteBuildErrorAsserter, err error) {
				fe := a.getFileError(err)
				a.c.Assert(fe.Position().LineNumber, qt.Equals, 7)
				a.c.Assert(fe.Position().ColumnNumber, qt.Equals, 10)
				a.assertErrorMessage(`"content/myyaml.md:7:10": failed to extract shortcode: template for shortcode "nono" not found`, fe.Error())
			},
		},
		{
			name:     "Invalid YAML front matter",
			fileType: yamlcontent,
			fileFixer: func(content string) string {
				return `---
title: "My YAML Content"
foo bar
---
`
			},
			assertBuildError: func(a testSiteBuildErrorAsserter, err error) {
				a.assertLineNumber(3, err)
			},
		},
		{
			name:     "Invalid TOML front matter",
			fileType: tomlcontent,
			fileFixer: func(content string) string {
				return strings.Replace(content, "description = ", "description &", 1)
			},
			assertBuildError: func(a testSiteBuildErrorAsserter, err error) {
				fe := a.getFileError(err)
				a.c.Assert(fe.Position().LineNumber, qt.Equals, 6)
			},
		},
		{
			name:     "Invalid JSON front matter",
			fileType: jsoncontent,
			fileFixer: func(content string) string {
				return strings.Replace(content, "\"description\":", "\"description\"", 1)
			},
			assertBuildError: func(a testSiteBuildErrorAsserter, err error) {
				fe := a.getFileError(err)
				a.c.Assert(fe.Position().LineNumber, qt.Equals, 3)
			},
		},
		{
			// See https://github.com/gohugoio/hugo/issues/5327
			name:     "Panic in template Execute",
			fileType: single,
			fileFixer: func(content string) string {
				return strings.Replace(content, ".Title", ".Parent.Parent.Parent", 1)
			},

			assertBuildError: func(a testSiteBuildErrorAsserter, err error) {
				a.c.Assert(err, qt.Not(qt.IsNil))
				fe := a.getFileError(err)
				a.c.Assert(fe.Position().LineNumber, qt.Equals, 5)
				a.c.Assert(fe.Position().ColumnNumber, qt.Equals, 21)
			},
		},
	}

	for _, test := range tests {
		if test.name != "Invalid JSON front matter" {
			continue
		}
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			c := qt.New(t)
			errorAsserter := testSiteBuildErrorAsserter{
				c:    c,
				name: test.name,
			}

			b := newTestSitesBuilder(t).WithSimpleConfigFile()

			f := func(fileType, content string) string {
				if fileType != test.fileType {
					return content
				}
				return test.fileFixer(content)
			}

			b.WithTemplatesAdded("layouts/shortcodes/sc.html", f(shortcode, `SHORTCODE L1
SHORTCODE L2
SHORTCODE L3:
SHORTCODE L4: {{ .Page.Title }}
`))
			b.WithTemplatesAdded("layouts/_default/baseof.html", f(base, `BASEOF L1
BASEOF L2
BASEOF L3
BASEOF L4{{ if .Title }}{{ end }}
{{block "main" .}}This is the main content.{{end}}
BASEOF L6
`))

			b.WithTemplatesAdded("layouts/_default/single.html", f(single, `{{ define "main" }}
SINGLE L2:
SINGLE L3:
SINGLE L4:
SINGLE L5: {{ .Title }} {{ .Content }}
{{ end }}
`))

			b.WithTemplatesAdded("layouts/foo/single.html", f(single, `
SINGLE L2:
SINGLE L3:
SINGLE L4:
SINGLE L5: {{ .Title }} {{ .Content }}
`))

			b.WithContent("myyaml.md", f(yamlcontent, `---
title: "The YAML"
---

Some content.

         {{< sc >}}

Some more text.

The end.

`))

			b.WithContent("mytoml.md", f(tomlcontent, `+++
title = "The TOML"
p1 = "v"
p2 = "v"
p3 = "v"
description = "Descriptioon"
+++

Some content.


`))

			b.WithContent("myjson.md", f(jsoncontent, `{
	"title": "This is a title",
	"description": "This is a description."
}

Some content.


`))

			createErr := b.CreateSitesE()
			if test.assertCreateError != nil {
				test.assertCreateError(errorAsserter, createErr)
			} else {
				c.Assert(createErr, qt.IsNil)
			}

			if createErr == nil {
				buildErr := b.BuildE(BuildCfg{})
				if test.assertBuildError != nil {
					test.assertBuildError(errorAsserter, buildErr)
				} else {
					c.Assert(buildErr, qt.IsNil)
				}
			}
		})
	}

}

// Issue 9852
func TestErrorMinify(t *testing.T) {
	t.Parallel()

	files := `
-- config.toml --
minify = true

-- layouts/index.html --
<body>
<script>=;</script>
</body>

`

	b, err := NewIntegrationTestBuilder(
		IntegrationTestConfig{
			T:           t,
			TxtarString: files,
		},
	).BuildE()

	fe := herrors.UnwrapFileError(err)
	b.Assert(fe, qt.IsNotNil)
	b.Assert(fe.Position().LineNumber, qt.Equals, 2)
	b.Assert(fe.Position().ColumnNumber, qt.Equals, 9)
	b.Assert(fe.Error(), qt.Contains, "unexpected = in expression on line 2 and column 9")
	b.Assert(filepath.ToSlash(fe.Position().Filename), qt.Contains, "hugo-transform-error")
	b.Assert(os.Remove(fe.Position().Filename), qt.IsNil)

}

func TestErrorNested(t *testing.T) {
	t.Parallel()

	files := `
-- config.toml --
-- layouts/index.html --
line 1
12{{ partial "foo.html" . }}
line 4
line 5
-- layouts/partials/foo.html --
line 1
line 2
123{{ .ThisDoesNotExist }}
line 4
`

	b, err := NewIntegrationTestBuilder(
		IntegrationTestConfig{
			T:           t,
			TxtarString: files,
		},
	).BuildE()

	b.Assert(err, qt.IsNotNil)
	errors := herrors.UnwrapFileErrorsWithErrorContext(err)
	b.Assert(errors, qt.HasLen, 2)
	fmt.Println(errors[0])
	b.Assert(errors[0].Position().LineNumber, qt.Equals, 2)
	b.Assert(errors[0].Position().ColumnNumber, qt.Equals, 5)
	b.Assert(errors[0].Error(), qt.Contains, filepath.FromSlash(`"/layouts/index.html:2:5": execute of template failed`))
	b.Assert(errors[0].ErrorContext().Lines, qt.DeepEquals, []string{"line 1", "12{{ partial \"foo.html\" . }}", "line 4", "line 5"})
	b.Assert(errors[1].Position().LineNumber, qt.Equals, 3)
	b.Assert(errors[1].Position().ColumnNumber, qt.Equals, 6)
	b.Assert(errors[1].ErrorContext().Lines, qt.DeepEquals, []string{"line 1", "line 2", "123{{ .ThisDoesNotExist }}", "line 4"})

}

// https://github.com/gohugoio/hugo/issues/5375
func TestSiteBuildTimeout(t *testing.T) {
	if !htesting.IsCI() {
		//defer leaktest.CheckTimeout(t, 10*time.Second)()
	}

	b := newTestSitesBuilder(t)
	b.WithConfigFile("toml", `
timeout = 5
`)

	b.WithTemplatesAdded("_default/single.html", `
{{ .WordCount }}
`, "shortcodes/c.html", `
{{ range .Page.Site.RegularPages }}
{{ .WordCount }}
{{ end }}

`)

	for i := 1; i < 100; i++ {
		b.WithContent(fmt.Sprintf("page%d.md", i), `---
title: "A page"
---

{{< c >}}`)
	}

	b.CreateSites().BuildFail(BuildCfg{})
}
