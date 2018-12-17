package chezmoi

import (
	"errors"
	"os"
	"testing"
	"text/template"

	"github.com/d4l3k/messagediff"
	"github.com/twpayne/go-vfs/vfst"
)

func TestTargetStatePopulate(t *testing.T) {
	for _, tc := range []struct {
		name      string
		root      interface{}
		sourceDir string
		data      map[string]interface{}
		funcs     template.FuncMap
		want      *TargetState
	}{
		{
			name: "simple_file",
			root: map[string]string{
				"/foo": "bar",
			},
			sourceDir: "/",
			want: &TargetState{
				TargetDir: "/",
				Umask:     0,
				SourceDir: "/",
				Entries: map[string]Entry{
					"foo": &File{
						sourceName: "foo",
						targetName: "foo",
						Perm:       0666,
						contents:   []byte("bar"),
					},
				},
			},
		},
		{
			name: "dot_file",
			root: map[string]string{
				"/dot_foo": "bar",
			},
			sourceDir: "/",
			want: &TargetState{
				TargetDir: "/",
				Umask:     0,
				SourceDir: "/",
				Entries: map[string]Entry{
					".foo": &File{
						sourceName: "dot_foo",
						targetName: ".foo",
						Perm:       0666,
						contents:   []byte("bar"),
					},
				},
			},
		},
		{
			name: "private_file",
			root: map[string]string{
				"/private_foo": "bar",
			},
			sourceDir: "/",
			want: &TargetState{
				TargetDir: "/",
				Umask:     0,
				SourceDir: "/",
				Entries: map[string]Entry{
					"foo": &File{
						sourceName: "private_foo",
						targetName: "foo",
						Perm:       0600,
						contents:   []byte("bar"),
					},
				},
			},
		},
		{
			name: "file_in_subdir",
			root: map[string]string{
				"/foo/bar": "baz",
			},
			sourceDir: "/",
			want: &TargetState{
				TargetDir: "/",
				Umask:     0,
				SourceDir: "/",
				Entries: map[string]Entry{
					"foo": &Dir{
						sourceName: "foo",
						targetName: "foo",
						Perm:       0777,
						Entries: map[string]Entry{
							"bar": &File{
								sourceName: "foo/bar",
								targetName: "foo/bar",
								Perm:       0666,
								contents:   []byte("baz"),
							},
						},
					},
				},
			},
		},
		{
			name: "file_in_private_dot_subdir",
			root: map[string]string{
				"/private_dot_foo/bar": "baz",
			},
			sourceDir: "/",
			want: &TargetState{
				TargetDir: "/",
				Umask:     0,
				SourceDir: "/",
				Entries: map[string]Entry{
					".foo": &Dir{
						sourceName: "private_dot_foo",
						targetName: ".foo",
						Perm:       0700,
						Entries: map[string]Entry{
							"bar": &File{
								sourceName: "private_dot_foo/bar",
								targetName: ".foo/bar",
								Perm:       0666,
								contents:   []byte("baz"),
							},
						},
					},
				},
			},
		},
		{
			name: "template_dot_file",
			root: map[string]string{
				"/dot_gitconfig.tmpl": "[user]\n\temail = {{.Email}}\n",
			},
			sourceDir: "/",
			data: map[string]interface{}{
				"Email": "user@example.com",
			},
			want: &TargetState{
				TargetDir: "/",
				Umask:     0,
				SourceDir: "/",
				Data: map[string]interface{}{
					"Email": "user@example.com",
				},
				Entries: map[string]Entry{
					".gitconfig": &File{
						sourceName: "dot_gitconfig.tmpl",
						targetName: ".gitconfig",
						Perm:       0666,
						Template:   true,
						contents:   []byte("[user]\n\temail = user@example.com\n"),
					},
				},
			},
		},
		{
			name: "symlink",
			root: map[string]interface{}{
				"/symlink_foo": "bar",
			},
			sourceDir: "/",
			want: &TargetState{
				TargetDir: "/",
				Umask:     0,
				SourceDir: "/",
				Entries: map[string]Entry{
					"foo": &Symlink{
						sourceName: "symlink_foo",
						targetName: "foo",
						linkname:   "bar",
					},
				},
			},
		},
		{
			name: "symlink_dot_foo",
			root: map[string]interface{}{
				"/symlink_dot_foo": "bar",
			},
			sourceDir: "/",
			want: &TargetState{
				TargetDir: "/",
				Umask:     0,
				SourceDir: "/",
				Entries: map[string]Entry{
					".foo": &Symlink{
						sourceName: "symlink_dot_foo",
						targetName: ".foo",
						linkname:   "bar",
					},
				},
			},
		},
		{
			name: "symlink_template",
			root: map[string]interface{}{
				"/symlink_foo.tmpl": "bar-{{ .host }}",
			},
			sourceDir: "/",
			data: map[string]interface{}{
				"host": "example.com",
			},
			want: &TargetState{
				TargetDir: "/",
				Umask:     0,
				SourceDir: "/",
				Data: map[string]interface{}{
					"host": "example.com",
				},
				Entries: map[string]Entry{
					"foo": &Symlink{
						sourceName: "symlink_foo.tmpl",
						targetName: "foo",
						Template:   true,
						linkname:   "bar-example.com",
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs, cleanup, err := vfst.NewTestFS(tc.root)
			defer cleanup()
			if err != nil {
				t.Fatalf("vfst.NewTestFS(_) == _, _, %v, want _, _, <nil>", err)
			}
			ts := NewTargetState("/", 0, tc.sourceDir, tc.data, tc.funcs)
			if err := ts.Populate(fs); err != nil {
				t.Fatalf("ts.Populate(%+v) == %v, want <nil>", fs, err)
			}
			if err := ts.Evaluate(); err != nil {
				t.Errorf("ts.Evaluate() == %v, want <nil>", err)
			}
			if diff, equal := messagediff.PrettyDiff(tc.want, ts); !equal {
				t.Errorf("ts.Populate(%+v) diff:\n%s\n", fs, diff)
			}
		})
	}
}

func TestReturnTemplateError(t *testing.T) {
	funcs := map[string]interface{}{
		"returnTemplateError": func() string {
			ReturnTemplateFuncError(errors.New("error"))
			return "foo"
		},
	}
	for name, dataString := range map[string]string{
		"syntax_error":         "{{",
		"unknown_field":        "{{ .Unknown }}",
		"unknown_func":         "{{ func }}",
		"func_returning_error": "{{ returnTemplateError }}",
	} {
		t.Run(name, func(t *testing.T) {
			ts := NewTargetState("/home/user", 0, "/home/user/.chezmoi", nil, funcs)
			if got, err := ts.executeTemplateData(name, []byte(dataString)); err == nil {
				t.Errorf("ts.executeTemplate(%q, %q) == %q, <nil>, want _, !<nil>", name, dataString, got)
			}
		})
	}
}

func TestEndToEnd(t *testing.T) {
	for _, tc := range []struct {
		name      string
		root      interface{}
		sourceDir string
		data      map[string]interface{}
		funcs     template.FuncMap
		targetDir string
		umask     os.FileMode
		tests     interface{}
	}{
		{
			name: "all",
			root: map[string]interface{}{
				"/home/user/.bashrc":                          "foo",
				"/home/user/replace_symlink":                  &vfst.Symlink{Target: "foo"},
				"/home/user/.chezmoi/dot_bashrc":              "bar",
				"/home/user/.chezmoi/.git/HEAD":               "HEAD",
				"/home/user/.chezmoi/dot_hgrc.tmpl":           "[ui]\nusername = {{ .name }} <{{ .email }}>\n",
				"/home/user/.chezmoi/empty.tmpl":              "{{ if false }}foo{{ end }}",
				"/home/user/.chezmoi/empty_foo":               "",
				"/home/user/.chezmoi/symlink_bar":             "empty",
				"/home/user/.chezmoi/symlink_replace_symlink": "bar",
			},
			sourceDir: "/home/user/.chezmoi",
			data: map[string]interface{}{
				"name":  "John Smith",
				"email": "hello@example.com",
			},
			targetDir: "/home/user",
			umask:     022,
			tests: []vfst.Test{
				vfst.TestPath("/home/user/.bashrc", vfst.TestModeIsRegular, vfst.TestContentsString("bar")),
				vfst.TestPath("/home/user/.hgrc", vfst.TestModeIsRegular, vfst.TestContentsString("[ui]\nusername = John Smith <hello@example.com>\n")),
				vfst.TestPath("/home/user/foo", vfst.TestModeIsRegular, vfst.TestContents(nil)),
				vfst.TestPath("/home/user/bar", vfst.TestModeType(os.ModeSymlink), vfst.TestSymlinkTarget("empty")),
				vfst.TestPath("/home/user/replace_symlink", vfst.TestModeType(os.ModeSymlink), vfst.TestSymlinkTarget("bar")),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs, cleanup, err := vfst.NewTestFS(tc.root)
			defer cleanup()
			if err != nil {
				t.Fatalf("vfst.NewTestFS(_) == _, _, %v, want _, _, <nil>", err)
			}
			ts := NewTargetState(tc.targetDir, tc.umask, tc.sourceDir, tc.data, tc.funcs)
			if err := ts.Populate(fs); err != nil {
				t.Fatalf("ts.Populate(%+v) == %v, want <nil>", fs, err)
			}
			if err := ts.Apply(fs, NewLoggingMutator(os.Stderr, NewFSMutator(fs, tc.targetDir))); err != nil {
				t.Fatalf("ts.Apply(fs, _) == %v, want <nil>", err)
			}
			vfst.RunTests(t, fs, "", tc.tests)
		})
	}
}
