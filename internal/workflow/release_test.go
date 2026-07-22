package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseUploadIdentifiesRepositoryWithoutCheckout(t *testing.T) {
	path := filepath.Join("..", "..", ".github", "workflows", "release.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	want := `--repo "${{ github.repository }}"`
	if !strings.Contains(string(content), want) {
		t.Fatalf("release upload must include %s because the attach job has no checkout", want)
	}
}
