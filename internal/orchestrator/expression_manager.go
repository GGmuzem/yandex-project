package orchestrator

import (
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/GGmuzem/yandex-project/pkg/models"
)

// ExpressionManager управляет выражениями и задачами
type ExpressionManager struct {
	mu           sync.Mutex
	expressions  map[string]*models.Expression
	tasks        map[int]*models.Task
	results      map[int]float64
	readyTasks   []models.Task
	processingTasks map[int]bool
	taskToExpr   map[int]string
	taskCounter  int
	exprCounter  int
}

// NewExpressionManager создает новый менеджер выражений
func NewExpressionManager() *ExpressionManager {
	return &ExpressionManager{
		expressions:    make(map[string]*models.Expression),
		tasks:          make(map[int]*models.Task),
		results:        make(map[int]float64),
		readyTasks:     []models.Task{},
		processingTasks: make(map[int]bool),
		taskToExpr:     make(map[int]string),
	}
}

// CreateExpression создает новое выражение и возвращает его ID
func (em *ExpressionManager) CreateExpression() string {
	em.mu.Lock()
	defer em.mu.Unlock()

	exprID := "expr id:" + strconv.Itoa(em.exprCounter)
	em.exprCounter++
	em.expressions[exprID] = &models.Expression{
		ID:     exprID,
		Status: "pending",
	}
	return exprID
}

// AddTasks добавляет задачи для выражения
func (em *ExpressionManager) AddTasks(exprID string, taskList []models.Task) {
	em.mu.Lock()
	defer em.mu.Unlock()

	log.Printf("Создание задач для выражения %s. Всего задач: %d", exprID, len(taskList))

	// Создаем карту зависимостей между задачами
	for _, t := range taskList {
		em.taskCounter++
		t.ID = em.taskCounter
		em.tasks[t.ID] = &t
		em.taskToExpr[t.ID] = exprID

		if em.checkTaskReady(t) {
			em.readyTasks = append(em.readyTasks, t)
			log.Printf("Готовая задача #%d для выражения %s добавлена в очередь", t.ID, exprID)
		}
	}
}

// GetTask возвращает готовую к выполнению задачу
func (em *ExpressionManager) GetTask() (models.Task, bool) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if len(em.readyTasks) == 0 {
		em.updateTaskReadiness()
		if len(em.readyTasks) == 0 {
			return models.Task{}, false
		}
	}

	// Ищем задачу, которая еще не в обработке
	taskIndex := -1
	for i, task := range em.readyTasks {
		if !em.processingTasks[task.ID] {
			taskIndex = i
			break
		}
	}

	// Если все задачи уже в обработке, возвращаем пустую задачу
	if taskIndex == -1 {
		return models.Task{}, false
	}

	// Получаем задачу и удаляем ее из очереди
	task := em.readyTasks[taskIndex]

	// Если задача находится в середине очереди, перемещаем последнюю задачу на ее место
	if taskIndex < len(em.readyTasks)-1 {
		em.readyTasks[taskIndex] = em.readyTasks[len(em.readyTasks)-1]
	}
	em.readyTasks = em.readyTasks[:len(em.readyTasks)-1]

	// Помечаем задачу как "в обработке"
	em.processingTasks[task.ID] = true

	return task, true
}

// AddResult добавляет результат выполнения задачи
func (em *ExpressionManager) AddResult(taskID int, result float64) {
	em.mu.Lock()
	defer em.mu.Unlock()

	// Снимаем отметку "в обработке" с задачи
	delete(em.processingTasks, taskID)

	// Сохраняем результат
	em.results[taskID] = result

	// Обрабатываем зависимости - это ключевой метод
	em.processTaskDependencies(taskID, result)

	// Обновляем готовые задачи
	em.updateTaskReadiness()

	// Обновляем статусы выражений
	em.updateExpressions()
}

// GetExpression возвращает выражение по ID
func (em *ExpressionManager) GetExpression(exprID string) (*models.Expression, bool) {
	em.mu.Lock()
	defer em.mu.Unlock()

	expr, exists := em.expressions[exprID]
	return expr, exists
}

// GetAllExpressions возвращает все выражения
func (em *ExpressionManager) GetAllExpressions() []models.Expression {
	em.mu.Lock()
	defer em.mu.Unlock()

	exprList := make([]models.Expression, 0, len(em.expressions))
	for _, expr := range em.expressions {
		exprList = append(exprList, *expr)
	}
	return exprList
}

// processTaskDependencies обрабатывает зависимости задач
func (em *ExpressionManager) processTaskDependencies(taskID int, taskResult float64) {
	resultStr := "result" + strconv.Itoa(taskID)
	dependentTasks := make(map[int]*models.Task)

	// Находим все зависимые задачи
	for id, taskPtr := range em.tasks {
		task := *taskPtr
		if task.Arg1 == resultStr || task.Arg2 == resultStr {
			dependentTasks[id] = taskPtr
			log.Printf("Задача #%d зависит от результата задачи #%d", id, taskID)
		}
	}

	log.Printf("Найдено %d зависимых задач от результата задачи #%d", len(dependentTasks), taskID)

	// Обрабатываем найденные зависимые задачи
	for id, taskPtr := range dependentTasks {
		task := *taskPtr
		log.Printf("Обработка зависимой задачи #%d: %s %s %s", id, task.Arg1, task.Operation, task.Arg2)

		// Заменяем ссылки на результат фактическим значением
		if task.Arg1 == resultStr {
			log.Printf("Обновление задачи #%d: arg1 изменен с %s на %f", id, task.Arg1, taskResult)
			task.Arg1 = strconv.FormatFloat(taskResult, 'f', -1, 64)
		}

		if task.Arg2 == resultStr {
			log.Printf("Обновление задачи #%d: arg2 изменен с %s на %f", id, task.Arg2, taskResult)
			task.Arg2 = strconv.FormatFloat(taskResult, 'f', -1, 64)
		}

		log.Printf("Задача #%d обновлена: %s %s %s", id, task.Arg1, task.Operation, task.Arg2)

		// Проверяем, стала ли задача готовой к выполнению
		if isTaskReady(task) {
			log.Printf("Задача #%d теперь готова к выполнению", id)

			// Проверяем, нет ли этой задачи уже в очереди готовых
			alreadyInQueue := false
			for _, readyTask := range em.readyTasks {
				if readyTask.ID == id {
					alreadyInQueue = true
					break
				}
			}

			if !alreadyInQueue && !em.processingTasks[id] {
				log.Printf("Задача #%d добавлена в очередь готовых задач", id)
				*taskPtr = task
				em.readyTasks = append(em.readyTasks, task)
			} else {
				log.Printf("Задача #%d уже находится в очереди готовых задач или в обработке", id)
			}
		} else {
			log.Printf("Задача #%d не готова к выполнению после обновления аргументов", id)
			*taskPtr = task
		}
	}
}

// updateExpressions обновляет статусы выражений
func (em *ExpressionManager) updateExpressions() {
	log.Println("updateExpressions: проверка статусов выражений")

	// Итерируем по всем выражениям
	for exprID, expr := range em.expressions {
		// Пропускаем уже выполненные выражения
		if expr.Status == "completed" {
			continue
		}

		log.Printf("updateExpressions: проверка выражения %s (статус %s)", exprID, expr.Status)

		// Собираем все задачи для этого выражения
		exprTaskIDs := []int{}
		for taskID, id := range em.taskToExpr {
			if id == exprID {
				exprTaskIDs = append(exprTaskIDs, taskID)
			}
		}

		// Если для выражения не найдено задач, возможно парсинг не прошел
		if len(exprTaskIDs) == 0 {
			continue
		}

		// Список всех активных задач
		activeTaskIDs := []int{}
		// Список задач в обработке
		processingTaskIDs := []int{}
		// Список задач с результатами
		completedTaskIDs := []int{}

		// Проверяем статус всех связанных задач
		for _, taskID := range exprTaskIDs {
			// Проверяем, существует ли задача в списке активных
			if _, exists := em.tasks[taskID]; exists {
				activeTaskIDs = append(activeTaskIDs, taskID)
			}

			// Проверяем, находится ли задача в процессе выполнения
			if em.processingTasks[taskID] {
				processingTaskIDs = append(processingTaskIDs, taskID)
			}

			// Проверяем, имеет ли задача сохраненный результат
			if _, hasResult := em.results[taskID]; hasResult {
				completedTaskIDs = append(completedTaskIDs, taskID)
			}
		}

		// Проверяем, есть ли задачи в очереди готовых задач
		readyTasksCount := 0
		for _, task := range em.readyTasks {
			if em.taskToExpr[task.ID] == exprID {
				readyTasksCount++
			}
		}

		// Если нет активных задач, нет задач в обработке, нет задач в очереди готовых,
		// но есть выполненные задачи, то выражение завершено
		if len(activeTaskIDs) == 0 && len(processingTaskIDs) == 0 && readyTasksCount == 0 && len(completedTaskIDs) > 0 {
			// Находим задачу с самым высоким ID - это должна быть последняя операция
			var lastTaskID int
			for _, taskID := range completedTaskIDs {
				if taskID > lastTaskID {
					lastTaskID = taskID
				}
			}

			// Получаем результат последней задачи
			if result, exists := em.results[lastTaskID]; exists {
				expr.Status = "completed"
				expr.Result = result
			}
		}
	}
}

// updateTaskReadiness обновляет список готовых задач
func (em *ExpressionManager) updateTaskReadiness() {
	// Проверяем все активные задачи
	for taskID, taskPtr := range em.tasks {
		task := *taskPtr

		// Проверяем, не находится ли задача уже в обработке
		if em.processingTasks[taskID] {
			continue
		}

		// Проверяем готовность задачи
		if em.checkTaskReady(task) {
			em.readyTasks = append(em.readyTasks, task)
		} else {
			// Проверяем, можем ли мы обновить аргументы задачи
			if strings.HasPrefix(task.Arg1, "result") {
				resultID := strings.TrimPrefix(task.Arg1, "result")
				if id, err := strconv.Atoi(resultID); err == nil {
					if result, exists := em.results[id]; exists {
						task.Arg1 = strconv.FormatFloat(result, 'f', -1, 64)
					}
				}
			}

			if strings.HasPrefix(task.Arg2, "result") {
				resultID := strings.TrimPrefix(task.Arg2, "result")
				if id, err := strconv.Atoi(resultID); err == nil {
					if result, exists := em.results[id]; exists {
						task.Arg2 = strconv.FormatFloat(result, 'f', -1, 64)
					}
				}
			}

			// Проверяем еще раз после обновления аргументов
			if em.checkTaskReady(task) {
				*taskPtr = task
				em.readyTasks = append(em.readyTasks, task)
			} else {
				*taskPtr = task
			}
		}
	}
}

// checkTaskReady проверяет, готова ли задача к выполнению
func (em *ExpressionManager) checkTaskReady(t models.Task) bool {
	// Проверяем первый аргумент
	log.Printf("Проверка готовности задачи #%d: arg1='%s', arg2='%s'", t.ID, t.Arg1, t.Arg2)

	// Проверяем, содержат ли аргументы ссылки на результаты других задач
	arg1HasResult := strings.HasPrefix(t.Arg1, "result")
	arg2HasResult := strings.HasPrefix(t.Arg2, "result")

	if arg1HasResult {
		log.Printf("Задача #%d не готова: arg1='%s' ссылается на результат другой задачи", t.ID, t.Arg1)
		return false
	}

	if arg2HasResult {
		log.Printf("Задача #%d не готова: arg2='%s' ссылается на результат другой задачи", t.ID, t.Arg2)
		return false
	}

	// Если дошли сюда, значит оба аргумента содержат числовые значения
	// Проверяем, что они действительно числа
	_, err1 := strconv.ParseFloat(t.Arg1, 64)
	if err1 != nil {
		log.Printf("Задача #%d не готова: arg1='%s' не является числом", t.ID, t.Arg1)
		return false
	}

	_, err2 := strconv.ParseFloat(t.Arg2, 64)
	if err2 != nil {
		log.Printf("Задача #%d не готова: arg2='%s' не является числом", t.ID, t.Arg2)
		return false
	}

	// Проверка на деление на ноль
	if t.Operation == "/" {
		arg2, _ := strconv.ParseFloat(t.Arg2, 64)
		if arg2 == 0 {
			log.Printf("Задача #%d не готова: попытка деления на ноль", t.ID)
			return false
		}
	}

	log.Printf("Задача #%d готова к выполнению", t.ID)
	return true
}

// Глобальный экземпляр ExpressionManager
var ExprManager = NewExpressionManager()
