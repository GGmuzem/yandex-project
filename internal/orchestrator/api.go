package orchestrator

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/GGmuzem/yandex-project/pkg/models"
)

// Статусы задач
const (
	TaskStatusNew        = 0 // Новая задача
	TaskStatusReady      = 1 // Готова к выполнению
	TaskStatusInProgress = 2 // В процессе выполнения
	TaskStatusCompleted  = 3 // Выполнена
)

var (
	expressions     = make(map[string]*models.Expression)
	tasks           = make(map[int]*models.Task)
	results         = make(map[int]float64)
	readyTasks      = []models.Task{}
	processingTasks = make(map[int]bool) // Карта для отслеживания задач, которые сейчас обрабатываются
	taskToExpr      = make(map[int]string)
	mu              sync.Mutex
	taskCounter     int
	exprCounter     int
)

func CalculateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var input struct {
		Expression string `json:"expression"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	log.Printf("Получено выражение для вычисления: %s", input.Expression)

	// Генерируем уникальный ID для выражения
	exprID := GenerateUniqueExpressionID()

	// Создаем новое выражение
	expressions[exprID] = &models.Expression{ID: exprID, Status: "pending"}

	// Парсим выражение и создаем задачи
	go func() {
		tasks := ParseExpression(input.Expression)
		log.Printf("Создано %d задач для выражения %s", len(tasks), exprID)

		// Миграция: добавляем задачи в глобальный менеджер
		log.Printf("МИГРАЦИЯ: Добавление задач из API в глобальный менеджер")
		Manager.AddExpression(exprID, tasks)

		// Мигрируем очередь готовых задач
		mu.Lock()
		log.Printf("МИГРАЦИЯ: Всего задач в локальной очереди: %d", len(readyTasks))
		for _, task := range readyTasks {
			// Проверяем, не находится ли задача уже в очереди
			found := false
			for _, rtask := range Manager.ReadyTasks {
				if rtask.ID == task.ID {
					found = true
					break
				}
			}

			if !found && !Manager.ProcessingTasks[task.ID] {
				Manager.ReadyTasks = append(Manager.ReadyTasks, task)
				log.Printf("МИГРАЦИЯ: Задача #%d добавлена в очередь глобального менеджера", task.ID)
			}
		}
		mu.Unlock()

		// Дополнительная проверка состояния
		log.Printf("Состояние после миграции:")
		log.Printf("- Всего задач в Manager.Tasks: %d", len(Manager.Tasks))
		log.Printf("- Всего задач в Manager.ReadyTasks: %d", len(Manager.ReadyTasks))

		// Проверяем содержимое очереди
		for i, task := range Manager.ReadyTasks {
			log.Printf("- Задача #%d в очереди Manager.ReadyTasks: ID=%d, %s %s %s",
				i, task.ID, task.Arg1, task.Operation, task.Arg2)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
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

	w.Header().Set("Content-Type", "application/json")
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]models.Expression{"expression": *expr})
}

func TaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Используем глобальный Manager для получения задач
		Manager.mu.Lock()
		if len(Manager.ReadyTasks) == 0 {
			Manager.mu.Unlock()
			log.Printf("TaskHandler GET: Нет готовых задач")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Ищем задачу, которая еще не в обработке
		taskIndex := -1
		for i, task := range Manager.ReadyTasks {
			if !Manager.ProcessingTasks[task.ID] {
				taskIndex = i
				break
			}
		}

		// Если все задачи уже в обработке, возвращаем 404
		if taskIndex == -1 {
			Manager.mu.Unlock()
			log.Printf("TaskHandler GET: Все задачи уже в обработке")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Получаем задачу и удаляем ее из очереди
		task := Manager.ReadyTasks[taskIndex]

		log.Printf("TaskHandler GET: Найдена задача #%d: %s %s %s",
			task.ID, task.Arg1, task.Operation, task.Arg2)

		// Если задача находится в середине очереди, перемещаем последнюю задачу на ее место
		// и усекаем очередь
		if taskIndex < len(Manager.ReadyTasks)-1 {
			Manager.ReadyTasks[taskIndex] = Manager.ReadyTasks[len(Manager.ReadyTasks)-1]
		}
		Manager.ReadyTasks = Manager.ReadyTasks[:len(Manager.ReadyTasks)-1]

		// Помечаем задачу как "в обработке"
		Manager.ProcessingTasks[task.ID] = true

		Manager.mu.Unlock()

		log.Printf("TaskHandler GET: Отправляем задачу #%d агенту: %s %s %s",
			task.ID, task.Arg1, task.Operation, task.Arg2)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]models.Task{"task": task})
		return
	}

	if r.Method == http.MethodPost {
		var result models.TaskResult
		if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
			log.Printf("TaskHandler POST: ошибка декодирования тела запроса: %v", err)
			http.Error(w, "Invalid data", http.StatusUnprocessableEntity)
			return
		}

		log.Printf("TaskHandler POST: получен результат для задачи #%d: %f", result.ID, result.Result)

		// Используем метод AddResult из Manager для обработки результата
		success := Manager.AddResult(result)

		if success {
			log.Printf("TaskHandler POST: результат для задачи #%d успешно обработан", result.ID)
		} else {
			log.Printf("TaskHandler POST: ошибка обработки результата для задачи #%d", result.ID)
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// Функция для рекурсивной обработки зависимостей задач
func processTaskDependencies(taskID int, taskResult float64) {
	log.Printf("Обработка зависимостей для задачи #%d с результатом %f (resultStr=result%d)", taskID, taskResult, taskID)

	resultStr := "result" + strconv.Itoa(taskID)
	dependentTasks := make(map[int]*models.Task)

	// Находим все зависимые задачи
	for id, taskPtr := range tasks {
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
		if apiIsTaskReady(task) {
			log.Printf("Задача #%d теперь готова к выполнению", id)

			// Проверяем, нет ли этой задачи уже в очереди готовых
			alreadyInQueue := false
			for _, readyTask := range readyTasks {
				if readyTask.ID == id {
					alreadyInQueue = true
					break
				}
			}

			if !alreadyInQueue && !processingTasks[id] {
				log.Printf("Задача #%d добавлена в очередь готовых задач", id)
				*taskPtr = task
				readyTasks = append(readyTasks, task)
			} else {
				log.Printf("Задача #%d уже находится в очереди готовых задач или в обработке", id)
			}
		} else {
			log.Printf("Задача #%d не готова к выполнению после обновления аргументов", id)
			*taskPtr = task
		}
	}

	// Проверяем очередь готовых задач
	log.Printf("Проверка очереди готовых задач после обработки зависимостей:")
	for _, task := range readyTasks {
		log.Printf("В очереди готовых: задача #%d: %s %s %s (выражение %s)",
			task.ID, task.Arg1, task.Operation, task.Arg2, taskToExpr[task.ID])
	}
}

// Проверяет, находится ли задача в готовом для выполнения состоянии
func apiIsTaskReady(t models.Task) bool {
	// Проверяем первый аргумент
	log.Printf("Проверка готовности задачи #%d: arg1='%s', arg2='%s'", t.ID, t.Arg1, t.Arg2)

	// Проверяем, содержат ли аргументы ссылки на результаты других задач
	arg1HasResult := IsResultRef(t.Arg1)
	arg2HasResult := IsResultRef(t.Arg2)

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

// Проверяет, находится ли задача в очереди готовых задач
func isTaskInQueue(task models.Task, queue []models.Task) bool {
	for _, t := range queue {
		if t.ID == task.ID {
			return true
		}
	}
	return false
}

func apiUpdateExpressions() {
	log.Println("apiUpdateExpressions: вызов UpdateExpressions из manager.go")
	UpdateExpressions()
}

// Проверяет, находится ли задача в процессе обработки
func apiIsTaskProcessing(taskID int) bool {
	return IsTaskProcessing(taskID)
}

func processTask(taskId int, result float64) error {
	mu.Lock()
	defer mu.Unlock()

	log.Printf("Обработка результата задачи #%d: %f", taskId, result)

	// Проверяем, существует ли задача с таким ID
	_, exists := tasks[taskId]
	if !exists {
		log.Printf("Задача #%d не найдена, но результат %f будет обработан", taskId, result)
	} else {
		log.Printf("Задача #%d найдена в системе", taskId)
	}

	// Сохраняем результат
	results[taskId] = result

	// Удаляем задачу из списка обрабатываемых
	delete(processingTasks, taskId)

	// Обработка зависимостей задач
	log.Printf("Начинаем обработку зависимостей для задачи #%d с результатом %f", taskId, result)
	processTaskDependencies(taskId, result)

	// Обновляем статусы всех выражений
	UpdateExpressions()

	return nil
}

func updateReadyTasks() {
	log.Printf("updateReadyTasks: начало обновления списка готовых задач")

	// Очищаем текущий список готовых задач
	readyTasks = []models.Task{}

	// Проверяем все активные задачи
	for taskID, taskPtr := range tasks {
		task := *taskPtr
		log.Printf("updateReadyTasks: проверка задачи #%d: %s %s %s", taskID, task.Arg1, task.Operation, task.Arg2)

		// Проверяем, не находится ли задача уже в обработке
		if processingTasks[taskID] {
			log.Printf("updateReadyTasks: задача #%d уже в обработке, пропускаем", taskID)
			continue
		}

		// Проверяем готовность задачи
		if apiIsTaskReady(task) {
			log.Printf("updateReadyTasks: задача #%d готова к выполнению", taskID)
			readyTasks = append(readyTasks, task)
		} else {
			log.Printf("updateReadyTasks: задача #%d не готова к выполнению", taskID)

			// Проверяем, можем ли мы обновить аргументы задачи
			if strings.HasPrefix(task.Arg1, "result") {
				resultID := strings.TrimPrefix(task.Arg1, "result")
				if id, err := strconv.Atoi(resultID); err == nil {
					if result, exists := results[id]; exists {
						log.Printf("updateReadyTasks: обновляем arg1 задачи #%d результатом задачи #%d: %f", taskID, id, result)
						task.Arg1 = strconv.FormatFloat(result, 'f', -1, 64)
					}
				}
			}

			if strings.HasPrefix(task.Arg2, "result") {
				resultID := strings.TrimPrefix(task.Arg2, "result")
				if id, err := strconv.Atoi(resultID); err == nil {
					if result, exists := results[id]; exists {
						log.Printf("updateReadyTasks: обновляем arg2 задачи #%d результатом задачи #%d: %f", taskID, id, result)
						task.Arg2 = strconv.FormatFloat(result, 'f', -1, 64)
					}
				}
			}

			// Проверяем еще раз после обновления аргументов
			if apiIsTaskReady(task) {
				log.Printf("updateReadyTasks: задача #%d стала готова после обновления аргументов", taskID)
				*taskPtr = task
				readyTasks = append(readyTasks, task)
			} else {
				log.Printf("updateReadyTasks: задача #%d все еще не готова после обновления аргументов", taskID)
				*taskPtr = task
			}
		}
	}

	log.Printf("updateReadyTasks: обновление завершено, готово задач: %d", len(readyTasks))
}
