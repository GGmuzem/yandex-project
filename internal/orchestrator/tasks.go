package orchestrator

import (
    "sync"
    "github.com/GGmuzem/yandex-project/pkg/models"
)

type TaskManager struct {
    Expressions map[string]*models.Expression
    Tasks       map[int]*models.Task
    Results     map[int]float64
    ReadyTasks  []models.Task
    TaskToExpr  map[int]string // Связь задачи с выражением
    mu          sync.Mutex
    taskCounter int
}

var Manager = TaskManager{
    Expressions: make(map[string]*models.Expression),
    Tasks:       make(map[int]*models.Task),
    Results:     make(map[int]float64),
    ReadyTasks:  []models.Task{},
    TaskToExpr:  make(map[int]string),
}

func (tm *TaskManager) AddExpression(exprID string, tasks []models.Task) {
    tm.mu.Lock()
    defer tm.mu.Unlock()

    tm.Expressions[exprID] = &models.Expression{ID: exprID, Status: "pending"}
    for _, t := range tasks {
        tm.taskCounter++
        t.ID = tm.taskCounter
        tm.Tasks[t.ID] = &t
        tm.TaskToExpr[t.ID] = exprID // Сохраняем связь задачи с выражением
        if isTaskReady(t) {
            tm.ReadyTasks = append(tm.ReadyTasks, t)
        }
    }
}

func (tm *TaskManager) GetTask() (models.Task, bool) {
    tm.mu.Lock()
    defer tm.mu.Unlock()

    if len(tm.ReadyTasks) == 0 {
        return models.Task{}, false
    }
    task := tm.ReadyTasks[0]
    tm.ReadyTasks = tm.ReadyTasks[1:]
    return task, true
}

func (tm *TaskManager) AddResult(result models.TaskResult) bool {
    tm.mu.Lock()
    defer tm.mu.Unlock()

    if _, exists := tm.Tasks[result.ID]; !exists {
        return false
    }
    tm.Results[result.ID] = result.Result
    delete(tm.Tasks, result.ID)
    tm.updateExpressions()
    return true
}

func (tm *TaskManager) updateExpressions() {
    for _, expr := range tm.Expressions {
        if expr.Status == "completed" {
            continue
        }
        allTasksDone := true
        for taskID := range tm.Tasks {
            if tm.containsTask(expr.ID, taskID) {
                allTasksDone = false
                break
            }
        }
        if allTasksDone && len(tm.Results) > 0 {
            expr.Status = "completed"
            // Упрощённо: берём последний результат как итог выражения
            for _, r := range tm.Results {
                expr.Result = r
            }
        }
    }
}

func (tm *TaskManager) containsTask(exprID string, taskID int) bool {
    tm.mu.Lock()
    defer tm.mu.Unlock()
    // Проверяем, относится ли задача к выражению
    return tm.TaskToExpr[taskID] == exprID
}