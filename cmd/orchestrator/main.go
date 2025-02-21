package main

import (
    "log"
    "net/http"
    "github.com/GGmuzem/yandex-project/internal/orchestrator"
)

func main() {
    http.HandleFunc("/api/v1/calculate", orchestrator.CalculateHandler)
    http.HandleFunc("/api/v1/expressions", orchestrator.ListExpressionsHandler)
    http.HandleFunc("/api/v1/expressions/", orchestrator.GetExpressionHandler)
    http.HandleFunc("/internal/task", orchestrator.TaskHandler)

    log.Println("Orchestrator starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}