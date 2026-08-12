package controller

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/naive"
)

func TestNaiveLogsRejectsOversizedRows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = []gin.Param{
		{Key: "tag", Value: "naive-test"},
		{Key: "rows", Value: "10001"},
	}

	(&NaiveController{}).logs(ctx)

	var response struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Success {
		t.Fatal("oversized log request unexpectedly succeeded")
	}
	if response.Msg != "invalid rows" {
		t.Fatalf("message = %q, want invalid rows", response.Msg)
	}
}

func TestDeleteBinaryRemovesNaiveArtifactsOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	binDir := t.TempDir()
	logDir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", binDir)
	t.Setenv("XUI_LOG_FOLDER", logDir)

	binary := naive.BinaryPath()
	staged := filepath.Join(binDir, ".naive-interrupted")
	firstLog := naive.LogPath("first")
	secondLog := naive.LogPath("second")
	unrelated := filepath.Join(logDir, "x-ui.log")
	for _, path := range []string{binary, staged, firstLog, secondLog, unrelated} {
		if err := os.WriteFile(path, []byte("test\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	(&NaiveController{}).deleteBinary(ctx)

	for _, path := range []string{binary, staged, firstLog, secondLog} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("Naive artifact remains at %s: %v", path, err)
		}
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatalf("unrelated log was removed: %v", err)
	}
}
