package consent

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFrom_Missing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "consent")
	if got := ReadFrom(path); got != Unknown {
		t.Errorf("missing file: got %v, want Unknown", got)
	}
}

func TestReadFrom_Granted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "consent")
	os.WriteFile(path, []byte("review_data = true\n"), 0644)
	if got := ReadFrom(path); got != Granted {
		t.Errorf("got %v, want Granted", got)
	}
}

func TestReadFrom_Denied(t *testing.T) {
	path := filepath.Join(t.TempDir(), "consent")
	os.WriteFile(path, []byte("review_data = false\n"), 0644)
	if got := ReadFrom(path); got != Denied {
		t.Errorf("got %v, want Denied", got)
	}
}

func TestReadFrom_WithComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "consent")
	os.WriteFile(path, []byte("# comment\n\nreview_data = true\n"), 0644)
	if got := ReadFrom(path); got != Granted {
		t.Errorf("got %v, want Granted", got)
	}
}

func TestReadFrom_UnknownValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "consent")
	os.WriteFile(path, []byte("review_data = maybe\n"), 0644)
	if got := ReadFrom(path); got != Denied {
		t.Errorf("non-true value: got %v, want Denied", got)
	}
}

func TestWriteTo_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	path := filepath.Join(dir, "consent")
	if err := WriteTo(path, true); err != nil {
		t.Fatal(err)
	}
	if got := ReadFrom(path); got != Granted {
		t.Errorf("after write true: got %v, want Granted", got)
	}
}

func TestWriteTo_Denied(t *testing.T) {
	path := filepath.Join(t.TempDir(), "consent")
	if err := WriteTo(path, false); err != nil {
		t.Fatal(err)
	}
	if got := ReadFrom(path); got != Denied {
		t.Errorf("after write false: got %v, want Denied", got)
	}
}

func TestCheckOrPrompt_AlreadyGranted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "consent")
	WriteTo(path, true)

	var buf bytes.Buffer
	err := checkOrPromptAt(path, "https://example.com", strings.NewReader(""), &buf)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for granted, got %q", buf.String())
	}
}

func TestCheckOrPrompt_AlreadyDenied(t *testing.T) {
	path := filepath.Join(t.TempDir(), "consent")
	WriteTo(path, false)

	var buf bytes.Buffer
	err := checkOrPromptAt(path, "https://example.com", strings.NewReader(""), &buf)
	if err == nil {
		t.Fatal("expected error for denied consent")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for denied, got %q", buf.String())
	}
}

func TestCheckOrPrompt_AcceptYes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "consent")

	var buf bytes.Buffer
	err := checkOrPromptAt(path, "https://example.com", strings.NewReader("y\n"), &buf)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if got := ReadFrom(path); got != Granted {
		t.Errorf("consent file: got %v, want Granted", got)
	}
}

func TestCheckOrPrompt_AcceptEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "consent")

	var buf bytes.Buffer
	err := checkOrPromptAt(path, "https://example.com", strings.NewReader("\n"), &buf)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if got := ReadFrom(path); got != Granted {
		t.Errorf("consent file: got %v, want Granted", got)
	}
}

func TestCheckOrPrompt_Decline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "consent")

	var buf bytes.Buffer
	err := checkOrPromptAt(path, "https://example.com", strings.NewReader("n\n"), &buf)
	if err == nil {
		t.Fatal("expected error for declined")
	}
	if got := ReadFrom(path); got != Denied {
		t.Errorf("consent file: got %v, want Denied", got)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error should mention file path, got %q", err.Error())
	}
}

func TestCheckOrPrompt_NoticeContainsURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "consent")

	var buf bytes.Buffer
	checkOrPromptAt(path, "https://reviewd.example.com", strings.NewReader("y\n"), &buf)
	if !strings.Contains(buf.String(), "https://reviewd.example.com") {
		t.Errorf("notice should contain target URL, got %q", buf.String())
	}
}
