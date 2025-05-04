package orchestrator

import (
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GGmuzem/yandex-project/pkg/models"
)

var exprIDCounter int64

// InitTaskManager инициализирует или сбрасывает глобальный менеджер задач
func InitTaskManager() {
	log.Println("Инициализация менеджера задач")
	// Глобальный экземпляр уже инициализирован при определении,
	// эта функция может использоваться для сброса или дополнительной настройки
	Manager.mu.Lock()
	defer Manager.mu.Unlock()

	// Сбрасываем счетчики при необходимости
	Manager.taskCounter = 0
	Manager.exprCounter = 0

	// Сбрасываем атомарный счетчик ID выражений
	atomic.StoreInt64(&exprIDCounter, 0)

	log.Println("Менеджер задач инициализирован")
}

// GenerateUniqueExpressionID генерирует уникальный ID для выражения с использованием временной метки
func GenerateUniqueExpressionID() string {
	// Атомарно увеличиваем счетчик
	id := atomic.AddInt64(&exprIDCounter, 1)
	// Добавляем timestamp для уникальности
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	return strconv.FormatInt(timestamp, 10) + "-" + strconv.FormatInt(id, 10)
}

// GetExpressionIDFromURL извлекает ID выражения из URL
func GetExpressionIDFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// TaskManager управляет задачами и выражениями
type TaskManager struct {
	Expressions     map[string]*models.Expression // Карта выражений: ID -> Expression
	Tasks           map[int]*models.Task          // Карта задач: ID -> Task
	Results         map[int]float64               // Карта результатов: task_id -> result
	ReadyTasks      []models.Task                 // Очередь готовых к выполнению задач
	ProcessingTasks map[int]bool                  // Карта задач в обработке: task_id -> true
	TaskToExpr      map[int]string                // Связь задачи с выражением
	mu              sync.Mutex
	taskCounter     int
	exprCounter     int
}

// Manager глобальный экземпляр TaskManager
var Manager = TaskManager{
	Expressions:     make(map[string]*models.Expression),
	Tasks:           make(map[int]*models.Task),
	Results:         make(map[int]float64),
	ReadyTasks:      []models.Task{},
	ProcessingTasks: make(map[int]bool),
	TaskToExpr:      make(map[int]string),
}

// глобальный мьютекс для синхронизации доступа к общим ресурсам
var globalMutex sync.Mutex

// GetExpressions возвращает карту выражений
func GetExpressions() map[string]*models.Expression {
	return Manager.Expressions
}

// GetTasks возвращает карту задач
func GetTasks() map[int]*models.Task {
	return Manager.Tasks
}

// GetReadyTasks возвращает список готовых задач
func GetReadyTasks() []models.Task {
	return Manager.ReadyTasks
}

// GetResults возвращает карту результатов
func GetResults() map[int]float64 {
	return Manager.Results
}

// GetProcessingTasks возвращает карту задач в обработке
func GetProcessingTasks() map[int]bool {
	return Manager.ProcessingTasks
}

// GetTaskToExpr возвращает карту соответствия задач и выражений
func GetTaskToExpr() map[int]string {
	return Manager.TaskToExpr
}

// UpdateExpressions экспортируемая функция для обновления статусов выражений
func UpdateExpressions() {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	log.Println("Обновление статусов выражений")

	for exprID, expr := range Manager.Expressions {
		// Пропускаем уже завершенные выражения
		if expr.Status == "completed" || expr.Status == "error" {
			continue
		}

		// Проверяем, остались ли невыполненные задачи для этого выражения
		tasksRemain := false
		for taskID, id := range Manager.TaskToExpr {
			if id == exprID {
				if _, ok := Manager.Tasks[taskID]; ok {
					tasksRemain = true
					log.Printf("Для выражения %s остались невыполненные задачи", exprID)
					break
				}
			}
		}

		// Если все задачи выполнены, то устанавливаем статус "completed"
		if !tasksRemain {
			log.Printf("Все задачи для выражения %s выполнены", exprID)

			// Ищем задачу с финальным результатом
			// Обычно это задача с самым большим ID
			finalTaskID := 0
			finalResult := 0.0
			finalFound := false

			for taskID, id := range Manager.TaskToExpr {
				if id == exprID && taskID > finalTaskID {
					if result, ok := Manager.Results[taskID]; ok {
						finalTaskID = taskID
						finalResult = result
						finalFound = true
					}
				}
			}

			if finalFound {
				expr.Status = "completed"
				expr.Result = finalResult
				log.Printf("Выражение %s завершено с результатом %f", exprID, finalResult)
			} else {
				log.Printf("Ошибка: не найден финальный результат для выражения %s", exprID)
				expr.Status = "error"
			}
		}
	}
}

// AddExpression добавляет новое выражение и его задачи
func (tm *TaskManager) AddExpression(exprID string, tasks []models.Task) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	log.Printf("AddExpression: Добавление выражения %s с %d задачами", exprID, len(tasks))

	// Создаем выражение, если его еще нет
	if _, exists := tm.Expressions[exprID]; !exists {
		tm.Expressions[exprID] = &models.Expression{ID: exprID, Status: "pending"}
		log.Printf("AddExpression: Создано новое выражение с ID=%s и статусом=pending", exprID)
	} else {
		log.Printf("AddExpression: Выражение %s уже существует", exprID)
	}

	for i, t := range tasks {
		tm.taskCounter++
		t.ID = tm.taskCounter
		// Явно устанавливаем ID выражения для каждой задачи
		t.ExpressionID = exprID

		log.Printf("AddExpression: Обработка задачи #%d (индекс %d): %s %s %s с ID выражения: %s",
			t.ID, i, t.Arg1, t.Operation, t.Arg2, t.ExpressionID)

		// Создаем копию задачи и сохраняем указатель
		taskCopy := t
		tm.Tasks[t.ID] = &taskCopy
		log.Printf("AddExpression: Задача #%d добавлена в карту задач", t.ID)

		tm.TaskToExpr[t.ID] = exprID // Сохраняем связь задачи с выражением
		log.Printf("AddExpression: Связь задачи #%d с выражением %s сохранена", t.ID, exprID)

		// Проверка готовности задачи и добавление в очередь
		if isTaskReady(t) {
			log.Printf("AddExpression: Задача #%d готова к выполнению, добавляем в очередь", t.ID)

			// Проверяем, не находится ли задача уже в обработке
			if !tm.ProcessingTasks[t.ID] {
				// Добавляем в очередь только если не в обработке
				tm.ReadyTasks = append(tm.ReadyTasks, t)
				log.Printf("AddExpression: Задача #%d добавлена в очередь готовых задач", t.ID)
			} else {
				log.Printf("AddExpression: Задача #%d уже находится в обработке, не добавляем в очередь", t.ID)
			}
		} else {
			log.Printf("AddExpression: Задача #%d не готова к выполнению", t.ID)
		}
	}

	// Проверяем содержимое менеджера после добавления
	log.Printf("AddExpression: Состояние менеджера:")
	log.Printf("AddExpression: Всего выражений: %d", len(tm.Expressions))
	log.Printf("AddExpression: Всего задач: %d", len(tm.Tasks))
	log.Printf("AddExpression: Всего в очереди готовых: %d", len(tm.ReadyTasks))

	// Выводим содержимое очереди готовых задач
	for i, task := range tm.ReadyTasks {
		log.Printf("AddExpression: Задача #%d в очереди: ID=%d, %s %s %s, ExprID=%s",
			i, task.ID, task.Arg1, task.Operation, task.Arg2, task.ExpressionID)
	}

	log.Printf("AddExpression: Добавлено выражение %s, всего задач: %d, в очереди готовых: %d",
		exprID, len(tasks), len(tm.ReadyTasks))
}

// GenerateExpressionID создает новый ID для выражения
func (tm *TaskManager) GenerateExpressionID() string {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.exprCounter++
	exprID := "expr id:" + strconv.Itoa(tm.exprCounter)

	return exprID
}

// GetTask возвращает задачу для выполнения агентом
func (tm *TaskManager) GetTask() (models.Task, bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Проверяем и обновляем готовые задачи
	log.Printf("GetTask: Проверка готовых задач, текущее количество: %d", len(tm.ReadyTasks))

	// Проходим по всем задачам и проверяем их готовность
	log.Printf("GetTask: Всего задач в системе: %d", len(tm.Tasks))
	for taskID, taskPtr := range tm.Tasks {
		log.Printf("GetTask: Проверка задачи #%d", taskID)
		if !tm.ProcessingTasks[taskID] { // Пропускаем задачи, которые уже в обработке
			log.Printf("GetTask: Задача #%d не в обработке, проверяем готовность", taskID)
			task := *taskPtr
			if isTaskReady(task) {
				log.Printf("GetTask: Задача #%d готова к выполнению", taskID)
				// Проверяем, не находится ли задача уже в очереди
				var found bool
				for _, readyTask := range tm.ReadyTasks {
					if readyTask.ID == task.ID {
						found = true
						log.Printf("GetTask: Задача #%d уже в очереди готовых", taskID)
						break
					}
				}
				if !found {
					log.Printf("GetTask: Добавляем готовую задачу #%d в очередь", task.ID)
					tm.ReadyTasks = append(tm.ReadyTasks, task)
				}
			} else {
				log.Printf("GetTask: Задача #%d не готова к выполнению", taskID)
			}
		} else {
			log.Printf("GetTask: Задача #%d уже в обработке, пропускаем", taskID)
		}
	}

	// Если после обновления очередь всё ещё пуста, возвращаем ошибку
	if len(tm.ReadyTasks) == 0 {
		log.Printf("GetTask: Нет готовых задач")
		return models.Task{}, false
	}

	// Берём первую готовую задачу
	task := tm.ReadyTasks[0]
	tm.ReadyTasks = tm.ReadyTasks[1:]

	// Отмечаем задачу как обрабатываемую
	tm.ProcessingTasks[task.ID] = true

	log.Printf("GetTask: Возвращаем задачу #%d для выполнения", task.ID)
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
		if task.Arg1 == resultStr {
			dependentTasksCount++
			log.Printf("TaskManager.AddResult: Задача #%d зависит от результата задачи #%d", taskID, result.ID)

			// Заменяем ссылку на результат на фактическое значение
			log.Printf("TaskManager.AddResult: Обновление задачи #%d: arg1 изменен с %s на %f",
				taskID, task.Arg1, result.Result)
			task.Arg1 = strconv.FormatFloat(result.Result, 'f', -1, 64)

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
		if task.Arg2 == resultStr {
			dependentTasksCount++
			log.Printf("TaskManager.AddResult: Задача #%d зависит от результата задачи #%d", taskID, result.ID)

			// Заменяем ссылку на результат на фактическое значение
			log.Printf("TaskManager.AddResult: Обновление задачи #%d: arg2 изменен с %s на %f",
				taskID, task.Arg2, result.Result)
			task.Arg2 = strconv.FormatFloat(result.Result, 'f', -1, 64)

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

	// Удаляем задачу из списка обрабатываемых
	delete(tm.ProcessingTasks, result.ID)

	// Обновляем статусы выражений
	log.Printf("TaskManager.AddResult: Обновляем статусы выражений")
	for _, expr := range tm.Expressions {
		// Проверяем все задачи выражения
		allCompleted := true
		for taskID := range tm.Tasks {
			if tm.TaskToExpr[taskID] == expr.ID {
				allCompleted = false
				break
			}
		}
		if allCompleted {
			expr.Status = "completed"
			log.Printf("TaskManager.AddResult: Выражение %s завершено", expr.ID)
		}
	}

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

// IsTaskProcessing проверяет, обрабатывается ли задача в данный момент
func IsTaskProcessing(taskID int) bool {
	_, exists := Manager.ProcessingTasks[taskID]
	return exists
}

// IsResultRef проверяет, является ли аргумент ссылкой на результат
func IsResultRef(arg string) bool {
	return isResultRef(arg)
}

// isResultRef проверяет, является ли аргумент ссылкой на результат (локальная версия)
func isResultRef(arg string) bool {
	return strings.HasPrefix(arg, "result")
}

// isTaskReady проверяет, готова ли задача к выполнению:
func isTaskReady(task models.Task) bool {
	// Проверяем первый аргумент
	if strings.HasPrefix(task.Arg1, "result") {
		id, err := strconv.Atoi(strings.TrimPrefix(task.Arg1, "result"))
		if err != nil {
			log.Printf("isTaskReady: неверный ID результата в Arg1 задачи #%d: %v", task.ID, err)
			return false
		}
		if _, exists := Manager.Results[id]; !exists {
			return false
		}
	} else {
		if _, err := strconv.ParseFloat(task.Arg1, 64); err != nil {
			return false
		}
	}
	// Проверяем второй аргумент
	if strings.HasPrefix(task.Arg2, "result") {
		id, err := strconv.Atoi(strings.TrimPrefix(task.Arg2, "result"))
		if err != nil {
			log.Printf("isTaskReady: неверный ID результата в Arg2 задачи #%d: %v", task.ID, err)
			return false
		}
		if _, exists := Manager.Results[id]; !exists {
			return false
		}
	} else {
		if _, err := strconv.ParseFloat(task.Arg2, 64); err != nil {
			return false
		}
	}
	// Проверка деления на ноль
	if task.Operation == "/" {
		val2, _ := strconv.ParseFloat(task.Arg2, 64)
		if val2 == 0 {
			return false
		}
	}
	return true
}

// ContainsTask проверяет, содержит ли выражение указанную задачу
func (tm *TaskManager) ContainsTask(exprID string, taskID int) bool {
	return tm.TaskToExpr[taskID] == exprID
}

// generateExpressionID генерирует уникальный ID для выражения
func generateExpressionID() string {
	return GenerateUniqueExpressionID()
}

// getExpressionIDFromURL извлекает ID выражения из URL (функция-алиас)
func getExpressionIDFromURL(url string) string {
	return GetExpressionIDFromURL(url)
}

// updateExpressions обновляет статусы выражений (функция-алиас)
func updateExpressions() {
	UpdateExpressions()
}
