package orchestrator

import (
    "encoding/json"
    "net/http"
    "strconv"
    "strings"
    "sync"
    "github.com/GGmuzem/yandex-project/pkg/models"
)

var (
    expressions = make(map[string]*models.Expression)
    tasks       = make(map[int]*models.Task)
    results     = make(map[int]float64)
    readyTasks  = []models.Task{}
    mu          sync.Mutex
    taskCounter int
    exprCounter int
)

func CalculateHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var input struct {
        Expression string `json:"expression"`
    }
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Expression == "" {
        http.Error(w, "Invalid data", http.StatusUnprocessableEntity)
        return
    }

    exprID := "expr" + strconv.Itoa(exprCounter)
    exprCounter++
    mu.Lock()
    expressions[exprID] = &models.Expression{ID: exprID, Status: "pending"}
    mu.Unlock()

    go func() {
        taskList := ParseExpression(input.Expression)
        mu.Lock()
        for _, t := range taskList {
            taskCounter++
            t.ID = taskCounter
            tasks[t.ID] = &t
            if isTaskReady(t) {
                readyTasks = append(readyTasks, t)
            }
        }
        mu.Unlock()
    }()

    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]string{"id": exprID})
}

func ListExpressionsHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    mu.Lock()
    exprList := []models.Expression{}
    for _, expr := range expressions {
        exprList = append(exprList, *expr)
    }
    mu.Unlock()

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string][]models.Expression{"expressions": exprList})
}

func GetExpressionHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    id := strings.TrimPrefix(r.URL.Path, "/api/v1/expressions/")
    mu.Lock()
    expr, exists := expressions[id]
    mu.Unlock()

    if !exists {
        http.Error(w, "Expression not found", http.StatusNotFound)
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]models.Expression{"expression": *expr})
}

func TaskHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodGet {
        mu.Lock()
        if len(readyTasks) == 0 {
            mu.Unlock()
            w.WriteHeader(http.StatusNotFound)
            return
        }
        task := readyTasks[0]
        readyTasks = readyTasks[1:]
        mu.Unlock()

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]models.Task{"task": task})
        return
    }

    if r.Method == http.MethodPost {
        var result models.TaskResult
        if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
            http.Error(w, "Invalid data", http.StatusUnprocessableEntity)
            return
        }

        mu.Lock()
        if _, exists := tasks[result.ID]; !exists {
            mu.Unlock()
            http.Error(w, "Task not found", http.StatusNotFound)
            return
        }
        results[result.ID] = result.Result
        delete(tasks, result.ID)
        updateExpressions()
        mu.Unlock()

        w.WriteHeader(http.StatusOK)
        return
    }

    http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func updateExpressions() {
    for _, expr := range expressions {
        if expr.Status == "completed" {
            continue
        }
        allTasksDone := true
        for _, t := range tasks {
            if strings.Contains(expr.ID, strconv.Itoa(t.ID)) {
                allTasksDone = false
                break
            }
        }
        if allTasksDone {
            expr.Status = "completed"
            expr.Result = results[taskCounter] // Упрощённо, для одного выражения
        }
    }
}

func isTaskReady(t models.Task) bool {
    _, err1 := strconv.ParseFloat(t.Arg1, 64)
    _, err2 := strconv.ParseFloat(t.Arg2, 64)
    return err1 == nil && err2 == nil
}