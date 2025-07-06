package executor

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestLogTaskStart(t *testing.T) {
	tests := []struct {
		name   string
		script string
		cli    string
		want   []string // 期待される文字列が含まれているか
	}{
		{
			name:   "正常なログ記録",
			script: "test.sh",
			cli:    "claude",
			want:   []string{"START", "script: test.sh", "cli: claude"},
		},
		{
			name:   "特殊文字を含むスクリプト名",
			script: "test-script_v2.sh",
			cli:    "gemini",
			want:   []string{"START", "script: test-script_v2.sh", "cli: gemini"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			LogTaskStart(buf, tt.script, tt.cli)

			output := buf.String()
			for _, want := range tt.want {
				if !strings.Contains(output, want) {
					t.Errorf("LogTaskStart() output = %v, want to contain %v", output, want)
				}
			}

			// タイムスタンプ形式の確認（RFC3339形式）
			if !strings.HasPrefix(output, "[") || !strings.Contains(output, "T") || !strings.Contains(output, "]") {
				t.Errorf("LogTaskStart() timestamp format is incorrect: %v", output)
			}
		})
	}
}

func TestLogTaskSuccess(t *testing.T) {
	tests := []struct {
		name   string
		script string
		cli    string
		output string
		want   []string
	}{
		{
			name:   "正常な成功ログ",
			script: "test.sh",
			cli:    "claude",
			output: "output/test.md",
			want:   []string{"SUCCESS", "script: test.sh", "cli: claude", "output: output/test.md"},
		},
		{
			name:   "長い出力パス",
			script: "generate.sh",
			cli:    "gemini",
			output: "/very/long/path/to/output/directory/file.md",
			want:   []string{"SUCCESS", "script: generate.sh", "cli: gemini", "output: /very/long/path/to/output/directory/file.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			LogTaskSuccess(buf, tt.script, tt.cli, tt.output)

			output := buf.String()
			for _, want := range tt.want {
				if !strings.Contains(output, want) {
					t.Errorf("LogTaskSuccess() output = %v, want to contain %v", output, want)
				}
			}
		})
	}
}

func TestLogTaskFailure(t *testing.T) {
	tests := []struct {
		name       string
		script     string
		cli        string
		errorMsg   string
		retryCount int
		want       []string
	}{
		{
			name:       "初回失敗",
			script:     "test.sh",
			cli:        "claude",
			errorMsg:   "quota limit exceeded",
			retryCount: 1,
			want:       []string{"FAIL", "script: test.sh", "cli: claude", "error: quota limit exceeded", "retry: 1"},
		},
		{
			name:       "複数回リトライ",
			script:     "complex.sh",
			cli:        "gemini",
			errorMsg:   "timeout after 2m0s",
			retryCount: 3,
			want:       []string{"FAIL", "script: complex.sh", "cli: gemini", "error: timeout after 2m0s", "retry: 3"},
		},
		{
			name:       "エラーメッセージに特殊文字",
			script:     "error.sh",
			cli:        "claude",
			errorMsg:   "exit status 1: command 'test' failed",
			retryCount: 2,
			want:       []string{"FAIL", "script: error.sh", "cli: claude", "error: exit status 1: command 'test' failed", "retry: 2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			LogTaskFailure(buf, tt.script, tt.cli, tt.errorMsg, tt.retryCount)

			output := buf.String()
			for _, want := range tt.want {
				if !strings.Contains(output, want) {
					t.Errorf("LogTaskFailure() output = %v, want to contain %v", output, want)
				}
			}
		})
	}
}

func TestLogWithNilWriter(t *testing.T) {
	// nilのWriterでもパニックしないことを確認
	LogTaskStart(nil, "test.sh", "claude")
	LogTaskSuccess(nil, "test.sh", "claude", "output.md")
	LogTaskFailure(nil, "test.sh", "claude", "error", 1)
}

func TestLogTaskEntry(t *testing.T) {
	// TaskLogEntry構造体のフィールドが正しく設定されることを確認
	entry := TaskLogEntry{
		Timestamp: time.Now(),
		Status:    "START",
		Script:    "test.sh",
		CLI:       "claude",
		Output:    "",
		Error:     "",
		Retry:     0,
	}

	if entry.Script != "test.sh" {
		t.Errorf("TaskLogEntry.Script = %v, want %v", entry.Script, "test.sh")
	}
	if entry.CLI != "claude" {
		t.Errorf("TaskLogEntry.CLI = %v, want %v", entry.CLI, "claude")
	}
	if entry.Status != "START" {
		t.Errorf("TaskLogEntry.Status = %v, want %v", entry.Status, "START")
	}
}

func TestConcurrentLogging(t *testing.T) {
	// 並行アクセスでも安全に動作することを確認
	buf := &bytes.Buffer{}
	done := make(chan bool)

	// 複数のゴルーチンから同時にログを書き込む
	for i := 0; i < 10; i++ {
		go func(id int) {
			script := strings.ReplaceAll("test{{id}}.sh", "{{id}}", string(rune('0'+id)))
			LogTaskStart(buf, script, "claude")
			time.Sleep(10 * time.Millisecond)
			if id%2 == 0 {
				LogTaskSuccess(buf, script, "claude", "output.md")
			} else {
				LogTaskFailure(buf, script, "claude", "error", 1)
			}
			done <- true
		}(i)
	}

	// すべてのゴルーチンの完了を待つ
	for i := 0; i < 10; i++ {
		<-done
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// 20行（各ゴルーチンで2行）のログが記録されているはず
	if len(lines) != 20 {
		t.Errorf("Expected 20 log lines, got %d", len(lines))
	}

	// 各行が正しい形式であることを確認
	for i, line := range lines {
		if !strings.HasPrefix(line, "[") || !strings.Contains(line, "] ") {
			t.Errorf("Line %d has incorrect format: %s", i+1, line)
		}
	}
}