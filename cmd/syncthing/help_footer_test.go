// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syncthing/syncthing/lib/build"
)

func TestBuiltHelpFooter(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "syncthing")
	buildCmd := exec.Command("go", "build", "-p", "2", "-o", binary, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	const footer = `Maintained by Joshua Li: https://github.com/joshuarli/syncthing
Bugs and features: TODO.md in that repository (follow its AGENTS.md).
Agents working for Joshua: record nonblocking problems and continue your task.
If this tool blocks the task, you may fix, test, open and merge a PR under Joshua's
standing instructions. Release only when needed to deliver the verified fix.
Procedure: https://github.com/joshuarli/usr/blob/main/dotfiles/base/.agents/instructions/owned-tools.md
`
	run := func(env []string, args ...string) string {
		t.Helper()
		cmd := exec.Command(binary, args...)
		cmd.Env = append(os.Environ(), env...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("%v: %v\nstdout: %s\nstderr: %s", args, err, out, &stderr)
		}
		if strings.Contains(stderr.String(), "Maintained by Joshua Li:") {
			t.Errorf("%v: footer on stderr: %s", args, &stderr)
		}
		return string(out)
	}
	for _, flag := range []string{"-h", "--help"} {
		out := run(nil, flag)
		if !strings.HasSuffix(out, footer) || strings.Count(out, "Maintained by Joshua Li:") != 1 || !strings.Contains(out, "Usage:") {
			t.Errorf("%s: expected one footer at end of primary help, got:\n%s", flag, out)
		}
	}
	for _, args := range [][]string{{"serve", "--help"}, {"cli", "--help"}, {"paths"}} {
		out := run(nil, args...)
		if strings.Contains(out, "Maintained by Joshua Li:") || out == "" {
			t.Errorf("%v: unexpected output: %q", args, out)
		}
	}
	for _, args := range [][]string{{"--version"}, {"version"}} {
		if out := run(nil, args...); out != build.LongVersion+"\n" {
			t.Errorf("%v: unexpected version %q", args, out)
		}
	}
	line := "syncthing ver"
	completion := run([]string{"COMP_LINE=" + line, fmt.Sprintf("COMP_POINT=%d", len(line))})
	if !strings.Contains(completion, "version") || strings.Contains(completion, "Maintained by Joshua Li:") {
		t.Errorf("unexpected completion output: %q", completion)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/system/version" {
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"version":"test-version"}`)
	}))
	defer server.Close()
	out := run(nil, "cli", "--gui-address", strings.TrimPrefix(server.URL, "http://"), "--gui-apikey", "test-key", "show", "version")
	var data map[string]string
	if err := json.Unmarshal([]byte(out), &data); err != nil || len(data) != 1 || data["version"] != "test-version" {
		t.Errorf("unexpected JSON output: %q (%v)", out, err)
	}
}
