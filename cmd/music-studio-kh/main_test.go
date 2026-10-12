package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestProfileCommandsUseSixDigitPasswordAndHideSecrets(t *testing.T) {
	root, music := t.TempDir(), t.TempDir()
	var out bytes.Buffer
	e := run([]string{"--data", root, "profile", "create", "--name", "Personal", "--library", music, "--username", "Owner", "--port", "4747"}, strings.NewReader("123456\n"), &out)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(out.String(), "123456") {
		t.Fatal("password disclosed")
	}
	out.Reset()
	if e = run([]string{"--data", root, "profile", "list"}, strings.NewReader(""), &out); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out.String(), "Personal") || !strings.Contains(out.String(), "4747") {
		t.Fatal(out.String())
	}
}
