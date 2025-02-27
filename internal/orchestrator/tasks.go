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

	// Проверяем, есть ли задачи, которые зависят от этого результата
	resultStr := "result" + strconv.Itoa(result.ID)
	log.Printf("TaskManager.AddResult: Проверяем зависимые задачи от результата %s", resultStr)

	// Счетчик зависимых задач для логирования
	dependentTasksCount := 0

	for taskID, taskPtr := range tm.Tasks {
		// Создаем копию задачи
		task := *taskPtr

		// Если задача имеет аргумент, который является результатом текущей задачи
		if task.Arg1 == resultStr || task.Arg2 == resultStr {
			dependentTasksCount++
			log.Printf("TaskManager.AddResult: Задача #%d зависит от результата задачи #%d", taskID, result.ID)

			// Заменяем ссылку на результат на фактическое значение
			if task.Arg1 == resultStr {
				log.Printf("TaskManager.AddResult: Обновление задачи #%d: arg1 изменен с %s на %f",
					taskID, task.Arg1, result.Result)
				task.Arg1 = strconv.FormatFloat(result.Result, 'f', -1, 64)
			}
			if task.Arg2 == resultStr {
				log.Printf("TaskManager.AddResult: Обновление задачи #%d: arg2 изменен с %s на %f",
					taskID, task.Arg2, result.Result)
				task.Arg2 = strconv.FormatFloat(result.Result, 'f', -1, 64)
			}

			// Обновляем задачу
			*taskPtr = task
			log.Printf("TaskManager.AddResult: Задача #%d обновлена: %s %s %s",
				taskID, task.Arg1, task.Operation, task.Arg2)

			// Если теперь задача готова к выполнению, добавляем её в список готовых задач
			log.Printf("TaskManager.AddResult: Проверка готовности задачи #%d после обновления аргументов", taskID)
			if isTaskReady(task) {
				log.Printf("TaskManager.AddResult: Задача #%d теперь готова к выполнению", taskID)
				tm.ReadyTasks = append(tm.ReadyTasks, task)
				log.Printf("TaskManager.AddResult: Задача #%d добавлена в очередь готовых задач", taskID)
			} else {
				log.Printf("TaskManager.AddResult: Задача #%d не готова к выполнению после обновления аргументов", taskID)
			}
		}
	}

	// Удаляем выполненную задачу ПОСЛЕ обработки всех зависимостей
	delete(tm.Tasks, result.ID)

	// Проверяем выражения
	tm.updateExpressions()

	log.Printf("TaskManager.AddResult: Найдено %d зависимых задач от результата задачи #%d",
		dependentTasksCount, result.ID)

	// Дополнительно проверяем количество задач в очереди готовых
	log.Printf("TaskManager.AddResult: Проверка очереди готовых задач:")
	for _, task := range tm.ReadyTasks {
		log.Printf("TaskManager.AddResult: Готовая задача #%d: %s %s %s (выражение %s)",
			task.ID, task.Arg1, task.Operation, task.Arg2, tm.TaskToExpr[task.ID])
	}
	log.Printf("TaskManager.AddResult: Всего в очереди готовых задач: %d", len(tm.ReadyTasks))

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
			log.Printf("TaskManager.updateExpressions: выражение %s уже выполнено, пропускаем", exprID)
			continue
		}

		log.Printf("TaskManager.updateExpressions: проверка выражения %s (статус %s)", exprID, expr.Status)

		// Проверяем, совпадает ли exprID с ID выражения в объекте
		if exprID != expr.ID {
			log.Printf("TaskManager.updateExpressions: ВНИМАНИЕ! exprID из карты (%s) не совпадает с expr.ID (%s)", exprID, expr.ID)
			// Используем ID из объекта выражения
			exprID = expr.ID
		}

		// Считаем количество связанных задач для этого выражения
		relatedTasks := 0
		activeTasks := 0
		processingTasksCount := 0
		completedTasks := 0

		// Подсчитываем задачи по типам
		for taskID, exprID2 := range tm.TaskToExpr {
			if exprID2 == exprID {
				relatedTasks++

				if _, exists := tm.Tasks[taskID]; exists {
					// Задача активна (еще не выполнена)
					activeTasks++
					log.Printf("TaskManager.updateExpressions: выражение %s - задача #%d в списке активных", exprID, taskID)

					// Проверяем, обрабатывается ли она сейчас
					if tm.processingTask(taskID) {
						processingTasksCount++
						log.Printf("TaskManager.updateExpressions: выражение %s - задача #%d в процессе обработки", exprID, taskID)
					}
				} else if _, hasResult := tm.Results[taskID]; hasResult {
					// Задача выполнена, есть результат
					completedTasks++
					log.Printf("TaskManager.updateExpressions: выражение %s - задача #%d имеет сохраненный результат", exprID, taskID)
				}
			}
		}

		log.Printf("TaskManager.updateExpressions: для выражения %s найдено %d связанных задач", exprID, relatedTasks)
		log.Printf("TaskManager.updateExpressions: выражение %s - статистика: %d активных, %d в обработке, %d выполнено",
			exprID, activeTasks, processingTasksCount, completedTasks)

		// Проверяем задачи в очереди готовых для этого выражения
		readyTasksForExpr := 0
		for _, task := range tm.ReadyTasks {
			if tm.TaskToExpr[task.ID] == exprID {
				readyTasksForExpr++
				log.Printf("TaskManager.updateExpressions: выражение %s - задача #%d в очереди готовых", exprID, task.ID)
			}
		}
		log.Printf("TaskManager.updateExpressions: выражение %s имеет %d задач в очереди готовых", exprID, readyTasksForExpr)

		// Если нет активных задач и нет задач в обработке, но есть выполненные задачи,
		// и все задачи для выражения обработаны, то выражение выполнено
		if activeTasks == 0 && processingTasksCount == 0 && completedTasks > 0 && readyTasksForExpr == 0 {
			log.Printf("TaskManager.updateExpressions: все задачи для выражения %s выполнены", exprID)

			// Ищем финальный результат - берем результат последней задачи
			var lastTaskID int = -1
			var lastResult float64

			// Находим ID последней задачи для этого выражения
			for taskID, exprID2 := range tm.TaskToExpr {
				if exprID2 == exprID && taskID > lastTaskID {
					if _, hasResult := tm.Results[taskID]; hasResult {
						lastTaskID = taskID
						lastResult = tm.Results[taskID]
						log.Printf("TaskManager.updateExpressions: найден результат задачи #%d: %f для выражения %s",
							taskID, lastResult, exprID)
					}
				}
			}

			if lastTaskID != -1 {
				log.Printf("TaskManager.updateExpressions: обновляем статус выражения %s на completed, результат %f (от задачи #%d)",
					exprID, lastResult, lastTaskID)
				expr.Status = "completed"
				expr.Result = lastResult
			} else {
				log.Printf("TaskManager.updateExpressions: не удалось найти результат для выражения %s", exprID)
			}
		} else if activeTasks > 0 {
			// Если есть активные задачи, выводим их для отладки
			for taskID := range tm.Tasks {
				if tm.TaskToExpr[taskID] == exprID {
					log.Printf("Для выражения %s осталась невыполненная задача #%d", exprID, taskID)
				}
			}
		}
	}
}

// Проверяет, обрабатывается ли задача в данный момент
func (tm *TaskManager) processingTask(taskID int) bool {
	return isTaskProcessing(taskID)
}

// Проверяет, могут ли быть сразу вычислены аргументы задачи
func (tm *TaskManager) containsTask(exprID string, taskID int) bool {
	return tm.TaskToExpr[taskID] == exprID
}
