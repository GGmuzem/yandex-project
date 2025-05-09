package orchestrator

import (
	"log"
	"sort"
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

	// Сохраняем текущие результаты
	results := Manager.Results
	
	// Сбрасываем структуры данных
	Manager.Expressions = make(map[string]*models.Expression)
	Manager.Tasks = make(map[int]*models.Task)
	Manager.ReadyTasks = []models.Task{}
	Manager.ProcessingTasks = make(map[int]bool)
	Manager.TaskToExpr = make(map[int]string)
	
	// Восстанавливаем результаты
	Manager.Results = results

	// Сбрасываем счетчики при необходимости
	Manager.taskCounter = 0
	Manager.exprCounter = 0

	// Сбрасываем атомарный счетчик ID выражений
	atomic.StoreInt64(&exprIDCounter, 0)

	log.Printf("Менеджер задач инициализирован, сохранено %d результатов", len(Manager.Results))
	
	// Выводим сохраненные результаты для отладки
	for taskID, result := range Manager.Results {
		log.Printf("  Сохраненный результат задачи #%d: %f", taskID, result)
	}
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

		// Проверяем, остались ли задачи для этого выражения
		tasksRemaining := false
		for taskID := range Manager.Tasks {
			if Manager.TaskToExpr[taskID] == exprID {
				tasksRemaining = true
				break
			}
		}

		// Если задач больше нет, обновляем статус выражения
		if !tasksRemaining {
			log.Printf("UpdateExpressions: Все задачи для выражения %s выполнены", exprID)
			
			// Находим последний результат для этого выражения
			// Собираем все задачи, связанные с этим выражением
			relatedTasks := make(map[int]bool)
			for taskID, id := range Manager.TaskToExpr {
				if id == exprID {
					relatedTasks[taskID] = true
				}
			}

			// Теперь ищем задачу, которая не является аргументом для других задач
			finalTaskID := 0
			finalResult := 0.0
			finalFound := false

			// Сначала попробуем найти задачу, которая не используется как аргумент
			log.Printf("=== ОТЛАДКА UpdateExpressions: Всего задач для выражения %s: %d", exprID, len(relatedTasks))
			
			// Выводим все результаты для этого выражения
			log.Printf("=== ОТЛАДКА UpdateExpressions: Результаты для выражения %s:", exprID)
			for taskID := range relatedTasks {
				if result, ok := Manager.Results[taskID]; ok {
					log.Printf("=== ОТЛАДКА UpdateExpressions: Задача #%d = %f", taskID, result)
				}
			}
			
			for taskID := range relatedTasks {
				// Проверяем, есть ли результат для этой задачи
				result, resultExists := Manager.Results[taskID]
				if !resultExists {
					log.Printf("=== ОТЛАДКА UpdateExpressions: Задача #%d не имеет результата, пропускаю", taskID)
					continue // Пропускаем задачи без результатов
				}

				// Проверяем, используется ли эта задача как аргумент
				isUsedAsArg := false
				resultRef := "result" + strconv.Itoa(taskID)
				log.Printf("=== ОТЛАДКА UpdateExpressions: Проверяю, используется ли задача #%d как аргумент (resultRef=%s)", taskID, resultRef)
				
				for tid, task := range Manager.Tasks {
					if task.Arg1 == resultRef || task.Arg2 == resultRef {
						isUsedAsArg = true
						log.Printf("=== ОТЛАДКА UpdateExpressions: Задача #%d используется как аргумент в задаче #%d", taskID, tid)
						break
					}
				}

				// Если задача не используется как аргумент, это финальный результат
				if !isUsedAsArg {
					finalTaskID = taskID
					finalResult = result
					finalFound = true
					log.Printf("=== ОТЛАДКА UpdateExpressions: Найден финальный результат для выражения %s: задача #%d = %f", exprID, taskID, result)
					break
				} else {
					log.Printf("=== ОТЛАДКА UpdateExpressions: Задача #%d используется как аргумент, не может быть финальным результатом", taskID)
				}
			}

			// Если не нашли, используем задачу с самым большим ID как запасной вариант
			if !finalFound {
				for taskID := range relatedTasks {
					if result, ok := Manager.Results[taskID]; ok && taskID > finalTaskID {
						finalTaskID = taskID
						finalResult = result
						finalFound = true
					}
				}
				if finalFound {
					log.Printf("Использую задачу с самым большим ID #%d как финальный результат для выражения %s: %f", finalTaskID, exprID, finalResult)
				}
			}

			if finalFound {
				// Устанавливаем статус и результат выражения
				expr.Status = "completed"
				expr.Result = finalResult
				log.Printf("UpdateExpressions: Выражение %s завершено с результатом задачи #%d: %f", 
					exprID, finalTaskID, finalResult)

				// Сохраняем результат в БД
				if DB != nil {
					err := DB.UpdateExpressionStatus(exprID, "completed", finalResult)
					if err != nil {
						log.Printf("UpdateExpressions: Ошибка при обновлении статуса выражения %s в БД: %v", exprID, err)
					} else {
						log.Printf("UpdateExpressions: Статус выражения %s обновлен в БД: completed, результат: %f", 
							exprID, finalResult)
					}
				}
			} else {
				log.Printf("UpdateExpressions: Ошибка: не найдены результаты для выражения %s", exprID)
				expr.Status = "error"

				// Обновляем статус в БД
				if DB != nil {
					err := DB.UpdateExpressionStatus(exprID, "error", 0)
					if err != nil {
						log.Printf("UpdateExpressions: Ошибка при обновлении статуса выражения %s в БД: %v", exprID, err)
					}
				}
			}
		} else {
			log.Printf("UpdateExpressions: Выражение %s еще не завершено", exprID)
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
	
	// Дополнительная проверка результатов
	log.Printf("GetTask: Всего результатов в системе: %d", len(tm.Results))
	for taskID, result := range tm.Results {
		log.Printf("GetTask: Результат задачи #%d: %f", taskID, result)
	}
	
	for taskID, taskPtr := range tm.Tasks {
		log.Printf("GetTask: Проверка задачи #%d", taskID)
		if !tm.ProcessingTasks[taskID] { // Пропускаем задачи, которые уже в обработке
			log.Printf("GetTask: Задача #%d не в обработке, проверяем готовность", taskID)
			task := *taskPtr
			
			// Проверяем аргументы на зависимости от результатов
			if strings.HasPrefix(task.Arg1, "result") {
				sourceTaskID, err := strconv.Atoi(strings.TrimPrefix(task.Arg1, "result"))
				if err == nil {
					if result, exists := tm.Results[sourceTaskID]; exists {
						// Заменяем ссылку на результат на фактическое значение
						task.Arg1 = strconv.FormatFloat(result, 'f', -1, 64)
						log.Printf("GetTask: Обновление задачи #%d: arg1 изменен с %s на %s",
							taskID, "result"+strconv.Itoa(sourceTaskID), task.Arg1)
						*taskPtr = task
					}
				}
			}
			if strings.HasPrefix(task.Arg2, "result") {
				sourceTaskID, err := strconv.Atoi(strings.TrimPrefix(task.Arg2, "result"))
				if err == nil {
					if result, exists := tm.Results[sourceTaskID]; exists {
						// Заменяем ссылку на результат на фактическое значение
						task.Arg2 = strconv.FormatFloat(result, 'f', -1, 64)
						log.Printf("GetTask: Обновление задачи #%d: arg2 изменен с %s на %s",
							taskID, "result"+strconv.Itoa(sourceTaskID), task.Arg2)
						*taskPtr = task
					}
				}
			}
			
			// После обновления аргументов проверяем готовность
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
		log.Printf("TaskManager.AddResult: Задача #%d не найдена в списке активных задач", result.ID)
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
	log.Printf("TaskManager.AddResult: Проверяем зависимые задачи от результата %s = %f", resultStr, result.Result)

	// Выводим все текущие результаты для отладки
	log.Printf("TaskManager.AddResult: Текущие результаты в системе:")
	for taskID, res := range tm.Results {
		log.Printf("  Результат задачи #%d: %f", taskID, res)
	}
	
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

	log.Printf("TaskManager.AddResult: Найдено %d зависимых задач от результата задачи #%d",
		dependentTasksCount, result.ID)

	// Удаляем задачу из списка обрабатываемых, но сохраняем в списке задач для проверки зависимостей
	delete(tm.ProcessingTasks, result.ID)

	// Проверяем, остались ли еще задачи для этого выражения
	tasksRemaining := false
	for taskID := range tm.Tasks {
		if tm.TaskToExpr[taskID] == exprID {
			tasksRemaining = true
			break
		}
	}

	// Если задач больше нет, обновляем статус выражения
	if !tasksRemaining && exprID != "" {
		log.Printf("TaskManager.AddResult: Все задачи для выражения %s выполнены", exprID)
		
		// Находим последний результат для этого выражения
		type taskInfo struct {
			id     int
			result float64
		}
		var tasks []taskInfo
		
		// Собираем все результаты для этого выражения
		for taskID, id := range tm.TaskToExpr {
			if id == exprID {
				if result, exists := tm.Results[taskID]; exists {
					tasks = append(tasks, taskInfo{id: taskID, result: result})
					log.Printf("TaskManager.AddResult: Найден результат задачи #%d: %f", taskID, result)
				}
			}
		}
		
		if len(tasks) > 0 {
			// Сортируем задачи по ID в убывающем порядке (самый большой ID будет первым)
			sort.Slice(tasks, func(i, j int) bool {
				return tasks[i].id > tasks[j].id
			})
			
			// Берем самую последнюю задачу (с самым большим ID)
			lastTask := tasks[0]
			
			// Обновляем статус выражения
			if expr, exists := tm.Expressions[exprID]; exists {
				expr.Status = "completed"
				expr.Result = lastTask.result
				log.Printf("TaskManager.AddResult: Выражение %s завершено с результатом последней задачи #%d: %f", 
					exprID, lastTask.id, lastTask.result)
				
				// Сохраняем результат в БД, если она доступна
				if DB != nil {
					err := DB.UpdateExpressionStatus(exprID, "completed", lastTask.result)
					if err != nil {
						log.Printf("TaskManager.AddResult: Ошибка при обновлении статуса выражения %s в БД: %v", exprID, err)
					} else {
						log.Printf("TaskManager.AddResult: Статус выражения %s обновлен в БД: completed, результат: %f", 
							exprID, lastTask.result)
					}
				}
			}
		} else {
			log.Printf("TaskManager.AddResult: Ошибка: не найдены результаты для выражения %s", exprID)
			
			// Обновляем статус выражения на ошибку
			if expr, exists := tm.Expressions[exprID]; exists {
				expr.Status = "error"
				
				// Обновляем статус в БД
				if DB != nil {
					err := DB.UpdateExpressionStatus(exprID, "error", 0)
					if err != nil {
						log.Printf("TaskManager.AddResult: Ошибка при обновлении статуса выражения %s в БД: %v", exprID, err)
					}
				}
			}
		}
	}
	
	// Проверяем количество задач в очереди готовых
	log.Printf("TaskManager.AddResult: Найдено %d зависимых задач от результата задачи #%d",
		dependentTasksCount, result.ID)

	// Дополнительно проверяем количество задач в очереди готовых
	log.Printf("TaskManager.AddResult: Проверка очереди готовых задач:")
	for _, task := range tm.ReadyTasks {
		log.Printf("TaskManager.AddResult: Готовая задача #%d: %s %s %s (выражение %s)",
			task.ID, task.Arg1, task.Operation, task.Arg2, tm.TaskToExpr[task.ID])
	}
	log.Printf("TaskManager.AddResult: Всего в очереди готовых задач: %d", len(tm.ReadyTasks))

	// Теперь, когда все зависимости обработаны и статус выражения обновлен,
	// можно безопасно удалить задачу из списка задач
	// Не удаляем из TaskToExpr, чтобы сохранить связь между задачами и выражениями
	delete(tm.Tasks, result.ID)
	log.Printf("TaskManager.AddResult: Задача #%d удалена из списка задач", result.ID)

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

// IsTaskReady проверяет, готова ли задача к выполнению
func IsTaskReady(task models.Task) bool {
	log.Printf("IsTaskReady: Проверка готовности задачи #%d: %s %s %s", task.ID, task.Arg1, task.Operation, task.Arg2)
	
	// Проверяем первый аргумент
	if strings.HasPrefix(task.Arg1, "result") {
		id, err := strconv.Atoi(strings.TrimPrefix(task.Arg1, "result"))
		if err != nil {
			log.Printf("IsTaskReady: неверный ID результата в Arg1 задачи #%d: %v", task.ID, err)
			return false
		}
		if _, exists := Manager.Results[id]; !exists {
			log.Printf("IsTaskReady: результат задачи #%d не найден для Arg1 задачи #%d", id, task.ID)
			return false
		} else {
			log.Printf("IsTaskReady: результат задачи #%d найден для Arg1 задачи #%d: %f", id, task.ID, Manager.Results[id])
		}
	} else {
		if _, err := strconv.ParseFloat(task.Arg1, 64); err != nil {
			log.Printf("IsTaskReady: Arg1 задачи #%d не является числом: %s", task.ID, task.Arg1)
			return false
		}
	}
	
	// Проверяем второй аргумент
	if strings.HasPrefix(task.Arg2, "result") {
		id, err := strconv.Atoi(strings.TrimPrefix(task.Arg2, "result"))
		if err != nil {
			log.Printf("IsTaskReady: неверный ID результата в Arg2 задачи #%d: %v", task.ID, err)
			return false
		}
		if _, exists := Manager.Results[id]; !exists {
			log.Printf("IsTaskReady: результат задачи #%d не найден для Arg2 задачи #%d", id, task.ID)
			return false
		} else {
			log.Printf("IsTaskReady: результат задачи #%d найден для Arg2 задачи #%d: %f", id, task.ID, Manager.Results[id])
		}
	} else {
		if _, err := strconv.ParseFloat(task.Arg2, 64); err != nil {
			log.Printf("IsTaskReady: Arg2 задачи #%d не является числом: %s", task.ID, task.Arg2)
			return false
		}
	}
	
	// Проверка деления на ноль
	if task.Operation == "/" {
		var val2 float64
		var err error
		
		if strings.HasPrefix(task.Arg2, "result") {
			id, _ := strconv.Atoi(strings.TrimPrefix(task.Arg2, "result"))
			val2 = Manager.Results[id]
		} else {
			val2, err = strconv.ParseFloat(task.Arg2, 64)
			if err != nil {
				log.Printf("IsTaskReady: ошибка при преобразовании Arg2 в число: %v", err)
				return false
			}
		}
		
		if val2 == 0 {
			log.Printf("IsTaskReady: ошибка - деление на ноль в задаче #%d", task.ID)
			return false
		}
	}
	
	log.Printf("IsTaskReady: задача #%d готова к выполнению", task.ID)
	return true
}

// isTaskReady - внутренняя версия функции для использования внутри пакета
func isTaskReady(task models.Task) bool {
	return IsTaskReady(task)
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
