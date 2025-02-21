package tests

import (
    "testing"
    "github.com/GGmuzem/yandex-project/internal/orchestrator"
)

func TestParseExpression(t *testing.T) {
    tasks := orchestrator.ParseExpression("2 + 2 * 2")
    if len(tasks) < 2 {
        t.Errorf("Expected at least 2 tasks, got %d", len(tasks))
    }
}