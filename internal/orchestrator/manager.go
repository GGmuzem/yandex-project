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

	// Сохраняем текущие результаты
	results := Manager.Results
	
	// Сбрасываем структуры данных
	Manager.Expressions = make(map[string]*models.Expression)
	Manager.Tasks = make(map[int]*models.Task)
	Manager.ReadyTasks = []models.Task{}
	Manager.ProcessingTasks = make(map[int]bool)
	Manager.TaskToExpr = make(map[int]string)
	Manager.TaskProcessingStartTime = make(map[int]time.Time)
	
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
	
	Manager.mu.Unlock()
	
	// Создаем тестовые задачи для отладки
	log.Println("Создание тестовых задач для отладки")
	
	// Создаем несколько тестовых задач
	for i := 0; i < 3; i++ {
		Manager.CreateTestTask()
		time.Sleep(100 * time.Millisecond) // Небольшая пауза между созданием задач
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
	Expressions            map[string]*models.Expression // Карта выражений: ID -> Expression
	Tasks                  map[int]*models.Task          // Карта задач: ID -> Task
	Results                map[int]float64               // Карта результатов: task_id -> result
	ReadyTasks             []models.Task                 // Очередь готовых к выполнению задач
	ProcessingTasks        map[int]bool                  // Карта задач в обработке: task_id -> true
	TaskToExpr             map[int]string                // Связь задачи с выражением
	TaskProcessingStartTime map[int]time.Time            // Время начала обработки задачи
	mu                     sync.Mutex
	taskCounter            int
	exprCounter            int
}

// NewTaskManager создает новый менеджер задач
func NewTaskManager() *TaskManager {
	return &TaskManager{
		mu:                     sync.Mutex{},
		Tasks:                  make(map[int]*models.Task),
		Expressions:            make(map[string]*models.Expression),
		TaskToExpr:             make(map[int]string),
		Results:                make(map[int]float64),
		ProcessingTasks:        make(map[int]bool),
		TaskProcessingStartTime: make(map[int]time.Time),
		ReadyTasks:             []models.Task{},
		taskCounter:            0,
	}
}

// CreateTestTask создает тестовую задачу для отладки
func (tm *TaskManager) CreateTestTask() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	// Увеличиваем счетчик задач
	tm.taskCounter++
	
	// Создаем тестовую задачу
	testTask := models.Task{
		ID:           tm.taskCounter,
		Arg1:         "5",
		Arg2:         "3",
		Operation:    "+",
		OperationTime: 1, // 1 секунда на выполнение
		ExpressionID: "test_expr_" + strconv.Itoa(tm.taskCounter),
	}
	
	// Сохраняем задачу в карте задач
	taskCopy := testTask
	tm.Tasks[testTask.ID] = &taskCopy
	
	// Создаем выражение, если его еще нет
	exprID := testTask.ExpressionID
	if _, exists := tm.Expressions[exprID]; !exists {
		tm.Expressions[exprID] = &models.Expression{ID: exprID, Status: "pending"}
	}
	
	// Сохраняем связь задачи с выражением
	tm.TaskToExpr[testTask.ID] = exprID
	
	// Добавляем задачу в очередь готовых задач
	tm.ReadyTasks = append(tm.ReadyTasks, testTask)
	
	log.Printf("=== TASK MANAGER: Создана тестовая задача #%d: %s %s %s, ExprID=%s", 
		testTask.ID, testTask.Arg1, testTask.Operation, testTask.Arg2, testTask.ExpressionID)
	log.Printf("=== TASK MANAGER: Всего задач: %d, в очереди готовых: %d", len(tm.Tasks), len(tm.ReadyTasks))
}

// Manager глобальный экземпляр TaskManager
var Manager = TaskManager{
	Expressions:            make(map[string]*models.Expression),
	Tasks:                  make(map[int]*models.Task),
	Results:                make(map[int]float64),
	ReadyTasks:             []models.Task{},
	ProcessingTasks:        make(map[int]bool),
	TaskToExpr:             make(map[int]string),
	TaskProcessingStartTime: make(map[int]time.Time),
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
		Manager.mu.Lock()
		for taskID := range Manager.Tasks {
			if Manager.TaskToExpr[taskID] == exprID {
				tasksRemaining = true
				break
			}
		}
		Manager.mu.Unlock()

		// Если задач больше нет, обновляем статус выражения
		if !tasksRemaining {
			log.Printf("UpdateExpressions: Все задачи для выражения %s выполнены", exprID)
			
			// Находим последний результат для этого выражения
			// Собираем все задачи, связанные с этим выражением
			relatedTasks := make(map[int]bool)
			Manager.mu.Lock()
			for taskID, id := range Manager.TaskToExpr {
				if id == exprID {
					relatedTasks[taskID] = true
				}
			}
			Manager.mu.Unlock()

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
				
				Manager.mu.Lock()
				for tid, task := range Manager.Tasks {
					if task.Arg1 == resultRef || task.Arg2 == resultRef {
						isUsedAsArg = true
						log.Printf("=== ОТЛАДКА UpdateExpressions: Задача #%d используется как аргумент в задаче #%d", taskID, tid)
						break
					}
				}
				Manager.mu.Unlock()

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
				// Собираем все задачи с результатами для этого выражения
				type taskWithResult struct {
					id     int
					result float64
				}
				var tasksWithResults []taskWithResult
				
				for taskID := range relatedTasks {
					if result, ok := Manager.Results[taskID]; ok {
						tasksWithResults = append(tasksWithResults, taskWithResult{id: taskID, result: result})
						log.Printf("=== ОТЛАДКА UpdateExpressions: Задача #%d имеет результат %f", taskID, result)
					}
				}
				
				// Сортируем задачи по ID в порядке убывания (самый большой ID будет первым)
				sort.Slice(tasksWithResults, func(i, j int) bool {
					return tasksWithResults[i].id > tasksWithResults[j].id
				})
				
				// Берем задачу с самым большим ID
				if len(tasksWithResults) > 0 {
					finalTaskID = tasksWithResults[0].id
					finalResult = tasksWithResults[0].result
					finalFound = true
					log.Printf("=== ОТЛАДКА UpdateExpressions: Выбрана задача с самым большим ID #%d с результатом %f для выражения %s", finalTaskID, finalResult, exprID)
				}
			}

			if finalFound {
				// Устанавливаем статус и результат выражения
				Manager.mu.Lock()
				expr.Status = "completed"
				expr.Result = finalResult
				log.Printf("UpdateExpressions: Выражение %s завершено с результатом задачи #%d: %f", 
					exprID, finalTaskID, finalResult)
				Manager.mu.Unlock()

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

	log.Printf("GetTask: Проверка наличия готовых задач, всего задач: %d, готовых: %d", 
		len(tm.Tasks), len(tm.ReadyTasks))

	// Обновляем список готовых задач
	tm.UpdateReadyTasksList()

	// Проверяем, есть ли готовые задачи
	if len(tm.ReadyTasks) == 0 {
		log.Printf("GetTask: Нет готовых задач")
		return models.Task{}, false
	}

	// Берем первую задачу из очереди
	task := tm.ReadyTasks[0]
	if len(tm.ReadyTasks) > 1 {
		tm.ReadyTasks = tm.ReadyTasks[1:]
	} else {
		tm.ReadyTasks = []models.Task{}
	}

	// Отмечаем задачу как обрабатываемую и сохраняем время начала обработки
	tm.ProcessingTasks[task.ID] = true
	tm.TaskProcessingStartTime[task.ID] = time.Now()

	log.Printf("GetTask: Возвращаем задачу #%d для выполнения", task.ID)
	return task, true
}

// UpdateReadyTasksList обновляет список готовых задач
func (tm *TaskManager) UpdateReadyTasksList() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Очищаем список готовых задач
	tm.ReadyTasks = []models.Task{}
	log.Printf("UpdateReadyTasksList: Список готовых задач очищен")

	// Проверяем все задачи на готовность
	for taskID, taskPtr := range tm.Tasks {
		// Пропускаем задачи, которые уже в обработке
		if tm.ProcessingTasks[taskID] {
			log.Printf("UpdateReadyTasksList: Задача #%d уже в обработке, пропускаем", taskID)
			continue
		}
		
		task := *taskPtr
		
		// Проверяем аргументы на зависимости от результатов
		arg1Ready := true
		arg2Ready := true
		
		// Проверяем первый аргумент
		if strings.HasPrefix(task.Arg1, "result") {
			sourceTaskID, err := strconv.Atoi(strings.TrimPrefix(task.Arg1, "result"))
			if err != nil {
				log.Printf("UpdateReadyTasksList: Ошибка при извлечении ID задачи из аргумента %s", task.Arg1)
				arg1Ready = false
			} else {
				// Проверяем, есть ли результат для этой задачи
				if result, exists := tm.Results[sourceTaskID]; exists {
					// Заменяем ссылку на результат на фактическое значение
					task.Arg1 = strconv.FormatFloat(result, 'f', -1, 64)
					log.Printf("UpdateReadyTasksList: Обновление задачи #%d: arg1 изменен с %s на %s",
						taskID, "result"+strconv.Itoa(sourceTaskID), task.Arg1)
					// Обновляем задачу в карте
					*taskPtr = task
				} else {
					log.Printf("UpdateReadyTasksList: Задача #%d не готова, поскольку результат задачи #%d еще не получен",
						taskID, sourceTaskID)
					arg1Ready = false
				}
			}
		}
		
		// Проверяем второй аргумент
		if strings.HasPrefix(task.Arg2, "result") {
			sourceTaskID, err := strconv.Atoi(strings.TrimPrefix(task.Arg2, "result"))
			if err != nil {
				log.Printf("UpdateReadyTasksList: Ошибка при извлечении ID задачи из аргумента %s", task.Arg2)
				arg2Ready = false
			} else {
				// Проверяем, есть ли результат для этой задачи
				if result, exists := tm.Results[sourceTaskID]; exists {
					// Заменяем ссылку на результат на фактическое значение
					task.Arg2 = strconv.FormatFloat(result, 'f', -1, 64)
					log.Printf("UpdateReadyTasksList: Обновление задачи #%d: arg2 изменен с %s на %s",
						taskID, "result"+strconv.Itoa(sourceTaskID), task.Arg2)
					// Обновляем задачу в карте
					*taskPtr = task
				} else {
					log.Printf("UpdateReadyTasksList: Задача #%d не готова, поскольку результат задачи #%d еще не получен",
						taskID, sourceTaskID)
					arg2Ready = false
				}
			}
		}
		
		// Если оба аргумента готовы, добавляем задачу в список готовых
		if arg1Ready && arg2Ready {
			tm.ReadyTasks = append(tm.ReadyTasks, task)
			log.Printf("UpdateReadyTasksList: Задача #%d добавлена в очередь готовых задач", taskID)
		}
	}
}

// AddResult добавляет результат задачи
func (tm *TaskManager) AddResult(result models.TaskResult) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Проверяем, что задача существует
	if _, exists := tm.Tasks[result.ID]; !exists {
		log.Printf("AddResult: Задача #%d не найдена", result.ID)
		return false
	}

	// Сохраняем результат в памяти
	tm.Results[result.ID] = result.Result
	log.Printf("AddResult: Результат задачи #%d сохранен: %f", result.ID, result.Result)

	// Удаляем задачу из списка задач в обработке
	delete(tm.ProcessingTasks, result.ID)
	delete(tm.TaskProcessingStartTime, result.ID)
	log.Printf("AddResult: Задача #%d удалена из списка задач в обработке", result.ID)

	// Обновляем список готовых задач
	tm.UpdateReadyTasksList()
	log.Printf("AddResult: Список готовых задач обновлен, всего готовых: %d", len(tm.ReadyTasks))

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

	expressions := make([]models.Expression, 0, len(tm.Expressions))
	for _, expr := range tm.Expressions {
		expressions = append(expressions, *expr)
	}
	return expressions
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

// isTaskReady проверяет, готова ли задача к выполнению (внутренняя реализация)
func isTaskReady(task models.Task) bool {
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

// IsTaskReady экспортируемая версия функции для проверки готовности задачи
func IsTaskReady(task models.Task) bool {
	return isTaskReady(task)
}

// ContainsTask проверяет, содержит ли выражение указанную задачу
func (tm *TaskManager) ContainsTask(exprID string, taskID int) bool {
	return tm.TaskToExpr[taskID] == exprID
}

// updateReadyTasksList обновляет список готовых задач
func (tm *TaskManager) updateReadyTasksList() {
	// Проверяем задачи, которые долго находятся в обработке
	now := time.Now()
	for taskID, startTime := range tm.TaskProcessingStartTime {
		if now.Sub(startTime) > 5*time.Minute {
			log.Printf("updateReadyTasksList: Задача #%d находится в обработке более 5 минут, освобождаем", taskID)
			delete(tm.ProcessingTasks, taskID)
			delete(tm.TaskProcessingStartTime, taskID)
		}
	}

	// Создаем новый список готовых задач
	readyTasks := []models.Task{}
	
	// Проходим по всем задачам и проверяем их готовность
	for taskID, taskPtr := range tm.Tasks {
		// Пропускаем задачи, которые уже в обработке
		if tm.ProcessingTasks[taskID] {
			log.Printf("updateReadyTasksList: Задача #%d уже в обработке, пропускаем", taskID)
			continue
		}
		
		task := *taskPtr
		
		// Проверяем аргументы на зависимости от результатов
		arg1Updated := false
		arg2Updated := false
		
		if strings.HasPrefix(task.Arg1, "result") {
			sourceTaskID, err := strconv.Atoi(strings.TrimPrefix(task.Arg1, "result"))
			if err == nil {
				if result, exists := tm.Results[sourceTaskID]; exists {
					// Заменяем ссылку на результат на фактическое значение
					task.Arg1 = strconv.FormatFloat(result, 'f', -1, 64)
					log.Printf("updateReadyTasksList: Обновление задачи #%d: arg1 изменен с %s на %s",
						taskID, "result"+strconv.Itoa(sourceTaskID), task.Arg1)
					arg1Updated = true
				}
			}
		}
		if strings.HasPrefix(task.Arg2, "result") {
			sourceTaskID, err := strconv.Atoi(strings.TrimPrefix(task.Arg2, "result"))
			if err == nil {
				if result, exists := tm.Results[sourceTaskID]; exists {
					// Заменяем ссылку на результат на фактическое значение
					task.Arg2 = strconv.FormatFloat(result, 'f', -1, 64)
					log.Printf("updateReadyTasksList: Обновление задачи #%d: arg2 изменен с %s на %s",
						taskID, "result"+strconv.Itoa(sourceTaskID), task.Arg2)
					arg2Updated = true
				}
			}
		}
		
		// Если аргументы были обновлены, обновляем задачу в карте
		if arg1Updated || arg2Updated {
			*taskPtr = task
		}
		
		// Проверяем готовность задачи
		if IsTaskReady(task) {
			log.Printf("updateReadyTasksList: Задача #%d готова к выполнению, добавляем в очередь", taskID)
			readyTasks = append(readyTasks, task)
		} else {
			log.Printf("updateReadyTasksList: Задача #%d не готова к выполнению", taskID)
		}
	}
	
	// Обновляем список готовых задач
	tm.ReadyTasks = readyTasks
	
	log.Printf("updateReadyTasksList: Обновлено готовых задач: %d", len(tm.ReadyTasks))

	// Детально логируем готовые задачи
	for i, task := range tm.ReadyTasks {
		log.Printf("updateReadyTasksList: Готовая задача #%d (индекс %d): %s %s %s", 
			task.ID, i, task.Arg1, task.Operation, task.Arg2)
	}
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
