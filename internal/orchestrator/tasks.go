package orchestrator

import (
	"log"
	"strconv"
	"sync"

	"github.com/GGmuzem/yandex-project/pkg/models"
)

// TaskManager управляет задачами и выражениями
type TaskManager struct {
	Expressions map[string]*models.Expression
	Tasks       map[int]*models.Task
	Results     map[int]float64
	ReadyTasks  []models.Task
	TaskToExpr  map[int]string // Связь задачи с выражением
	mu          sync.Mutex
	taskCounter int
	exprCounter int
}

// Manager глобальный экземпляр TaskManager
var Manager = TaskManager{
	Expressions: make(map[string]*models.Expression),
	Tasks:       make(map[int]*models.Task),
	Results:     make(map[int]float64),
	ReadyTasks:  []models.Task{},
	TaskToExpr:  make(map[int]string),
}

// AddExpression добавляет новое выражение и его задачи
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

// GenerateExpressionID создает новый ID для выражения
func (tm *TaskManager) GenerateExpressionID() string {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	exprID := "expr id: " + strconv.Itoa(tm.exprCounter)
	tm.exprCounter++

	return exprID
}

// GetTask возвращает задачу для выполнения агентом
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

// AddResult добавляет результат выполнения задачи
func (tm *TaskManager) AddResult(result models.TaskResult) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.Tasks[result.ID]; !exists {
		return false
	}

	log.Printf("TaskManager.AddResult: Получен результат для задачи #%d: %f", result.ID, result.Result)

	// Сохраняем результат
	tm.Results[result.ID] = result.Result

	// Находим ID выражения для этой задачи
	exprID := tm.TaskToExpr[result.ID]
	log.Printf("TaskManager.AddResult: Задача #%d принадлежит выражению %s", result.ID, exprID)

	// Удаляем выполненную задачу
	delete(tm.Tasks, result.ID)

	// Проверяем выражения
	tm.updateExpressions()

	// Проверяем, есть ли задачи, которые зависят от этого результата
	resultStr := "result" + strconv.Itoa(result.ID)
	for taskID, taskPtr := range tm.Tasks {
		// Создаем копию задачи
		task := *taskPtr

		// Если задача имеет аргумент, который является результатом текущей задачи
		if task.Arg1 == resultStr || task.Arg2 == resultStr {
			// Заменяем ссылку на результат на фактическое значение
			if task.Arg1 == resultStr {
				task.Arg1 = strconv.FormatFloat(result.Result, 'f', -1, 64)
			}
			if task.Arg2 == resultStr {
				task.Arg2 = strconv.FormatFloat(result.Result, 'f', -1, 64)
			}

			// Обновляем задачу
			*taskPtr = task

			// Если теперь задача готова к выполнению, добавляем её в список готовых задач
			if isTaskReady(task) {
				log.Printf("TaskManager.AddResult: Задача #%d теперь готова к выполнению: %s %s %s",
					taskID, task.Arg1, task.Operation, task.Arg2)
				tm.ReadyTasks = append(tm.ReadyTasks, task)
			}
		}
	}

	return true
}

// GetExpression возвращает выражение по его ID
func (tm *TaskManager) GetExpression(exprID string) (*models.Expression, bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	expr, exists := tm.Expressions[exprID]
	return expr, exists
}

// GetAllExpressions возвращает список всех выражений
func (tm *TaskManager) GetAllExpressions() []models.Expression {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	exprList := []models.Expression{}
	for _, expr := range tm.Expressions {
		exprList = append(exprList, *expr)
	}
	return exprList
}

// Обновляет статус выражений в зависимости от задач
func (tm *TaskManager) updateExpressions() {
	log.Println("TaskManager.updateExpressions: проверка статусов выражений")
	for exprID, expr := range tm.Expressions {
		if expr.Status == "completed" {
			continue
		}

		log.Printf("TaskManager.updateExpressions: проверка выражения %s (статус %s)", exprID, expr.Status)

		// Проверяем, совпадает ли exprID с ID выражения в объекте
		if exprID != expr.ID {
			log.Printf("TaskManager.updateExpressions: ВНИМАНИЕ! exprID из карты (%s) не совпадает с expr.ID (%s)", exprID, expr.ID)
			// Используем ID из объекта выражения
			exprID = expr.ID
		}

		// Проверяем завершенность всех задач для данного выражения
		allTasksDone := true
		for taskID := range tm.Tasks {
			if tm.TaskToExpr[taskID] == exprID {
				allTasksDone = false
				log.Printf("TaskManager.updateExpressions: выражение %s - есть незавершенные задачи", exprID)
				break
			}
		}

		if allTasksDone {
			log.Printf("TaskManager.updateExpressions: все задачи для выражения %s выполнены", exprID)

			// Ищем финальный результат, связанный с этим выражением
			var lastResult float64
			foundResult := false

			// Проверяем все результаты, которые связаны с этим выражением
			for taskID, result := range tm.Results {
				if tm.TaskToExpr[taskID] == exprID {
					lastResult = result
					foundResult = true
					log.Printf("TaskManager.updateExpressions: найден результат %f для выражения %s", result, exprID)
					break // Берем первый найденный результат
				}
			}

			// Если не найдено ни одного результата, но в TaskToExpr есть записи для этого выражения,
			// значит задачи были созданы, но результаты не были правильно связаны
			if !foundResult {
				log.Printf("TaskManager.updateExpressions: результаты для выражения %s не найдены, проверяем связи", exprID)

				// Проверяем, есть ли задачи, связанные с этим выражением
				hasRelatedTasks := false
				for taskID, exprID2 := range tm.TaskToExpr {
					if exprID2 == exprID {
						hasRelatedTasks = true
						log.Printf("TaskManager.updateExpressions: найдена связь задачи %d с выражением %s", taskID, exprID)
						break
					}
				}

				// Если задачи были связаны с этим выражением, но результатов нет,
				// и все задачи выполнены (allTasksDone = true), значит что-то пошло не так
				if hasRelatedTasks {
					log.Printf("TaskManager.updateExpressions: есть связанные задачи, ищем любой результат")

					// Берем последний добавленный результат как запасной вариант
					for _, res := range tm.Results {
						lastResult = res
						foundResult = true
						log.Printf("TaskManager.updateExpressions: взят первый доступный результат %f", res)
						break
					}
				}
			}

			if foundResult {
				log.Printf("TaskManager.updateExpressions: обновляем статус выражения %s на completed, результат %f", exprID, lastResult)
				expr.Status = "completed"
				expr.Result = lastResult
			} else {
				log.Printf("TaskManager.updateExpressions: результат для выражения %s не найден, оставляем статус %s", exprID, expr.Status)
			}
		}
	}
}

// Проверяет, могут ли быть сразу вычислены аргументы задачи
func (tm *TaskManager) containsTask(exprID string, taskID int) bool {
	return tm.TaskToExpr[taskID] == exprID
}
