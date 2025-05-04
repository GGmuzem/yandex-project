package orchestrator

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/GGmuzem/yandex-project/pkg/models"
)

// Статусы задач
const (
	TaskStatusNew        = 0 // Новая задача
	TaskStatusReady      = 1 // Готова к выполнению
	TaskStatusInProgress = 2 // В процессе выполнения
	TaskStatusCompleted  = 3 // Выполнена
)

// Используем глобальный Manager из manager.go

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

	// Парсим выражение и создаем задачи
	tasks := ParseExpression(input.Expression)
	log.Printf("Создано %d задач для выражения %s", len(tasks), exprID)

	// Добавляем задачи в глобальный менеджер
	log.Printf("Добавление задач в глобальный менеджер")
	Manager.AddExpression(exprID, tasks)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": exprID})
}

func ListExpressionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	Manager.mu.Lock()
	exprList := []models.Expression{}
	for _, expr := range Manager.Expressions {
		exprList = append(exprList, *expr)
	}
	Manager.mu.Unlock()

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
	Manager.mu.Lock()
	expr, exists := Manager.Expressions[id]
	Manager.mu.Unlock()

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
		// Используем метод GetTask из Manager для получения задачи
		task, found := Manager.GetTask()
		if !found {
			log.Printf("TaskHandler GET: Нет готовых задач")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		log.Printf("TaskHandler GET: Отправляем задачу #%d агенту: %s %s %s",
			task.ID, task.Arg1, task.Operation, task.Arg2)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := struct {
			Task models.Task `json:"task"`
		}{
			Task: task,
		}
		json.NewEncoder(w).Encode(response)
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
			log.Printf("TaskHandler POST: результат задачи #%d успешно обработан", result.ID)
			w.WriteHeader(http.StatusOK)
		} else {
			log.Printf("TaskHandler POST: ошибка обработки результата задачи #%d", result.ID)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// Функция для рекурсивной обработки зависимостей задач
func processTaskDependencies(taskID int, taskResult float64) {
	// Получаем ID выражения для этой задачи
	_, ok := Manager.TaskToExpr[taskID]
	if !ok {
		log.Printf("Не найдено выражение для задачи #%d", taskID)
		return
	}

	log.Printf("Обработка зависимостей для задачи #%d с результатом %f (resultStr=result%d)", taskID, taskResult, taskID)

	resultStr := "result" + strconv.Itoa(taskID)
	log.Printf("Ищем задачи, зависящие от задачи #%d (строка результата: %s)", taskID, resultStr)

	// Находим все зависимые задачи
	dependentTasks := make(map[int]*models.Task)
	for id, taskPtr := range Manager.Tasks {
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
			Manager.mu.Lock()
			alreadyInQueue := false
			for _, readyTask := range Manager.ReadyTasks {
				if readyTask.ID == id {
					alreadyInQueue = true
					break
				}
			}

			if !alreadyInQueue && !Manager.ProcessingTasks[id] {
				log.Printf("Задача #%d добавлена в очередь готовых задач", id)
				*taskPtr = task
				Manager.ReadyTasks = append(Manager.ReadyTasks, task)
			} else {
				log.Printf("Задача #%d уже находится в очереди готовых задач или в обработке", id)
			}
			Manager.mu.Unlock()
		} else {
			log.Printf("Задача #%d не готова к выполнению после обновления аргументов", id)
			*taskPtr = task
		}
	}

	// Проверяем очередь готовых задач
	log.Printf("Проверка очереди готовых задач после обработки зависимостей:")
	for _, task := range Manager.ReadyTasks {
		log.Printf("В очереди готовых: задача #%d: %s %s %s (выражение %s)",
			task.ID, task.Arg1, task.Operation, task.Arg2, Manager.TaskToExpr[task.ID])
	}
}

// Проверяет, находится ли задача в готовом для выполнения состоянии
func apiIsTaskReady(t models.Task) bool {
	log.Printf("Проверка готовности задачи #%d: %s %s %s", t.ID, t.Arg1, t.Operation, t.Arg2)

	// Если задача уже в обработке, она не готова
	if Manager.ProcessingTasks[t.ID] {
		log.Printf("Задача #%d уже в обработке", t.ID)
		return false
	}

	// Если задача уже в очереди готовых задач, она не готова
	if isTaskInQueue(t, Manager.ReadyTasks) {
		log.Printf("Задача #%d уже в очереди готовых задач", t.ID)
		return false
	}

	// Если хотя бы один аргумент является ссылкой на результат,
	// задача не готова
	if strings.HasPrefix(t.Arg1, "result") || strings.HasPrefix(t.Arg2, "result") {
		log.Printf("Задача #%d содержит ссылки на результаты", t.ID)
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
	log.Println("apiUpdateExpressions: обновление статусов выражений")
	updateReadyTasks()
}

// Проверяет, находится ли задача в процессе обработки
func apiIsTaskProcessing(taskID int) bool {
	return Manager.ProcessingTasks[taskID]
}

func processTask(taskId int, result float64) error {
	Manager.mu.Lock()
	defer Manager.mu.Unlock()

	log.Printf("Обработка результата задачи #%d: %f", taskId, result)

	// Проверяем, существует ли задача с таким ID
	_, exists := Manager.Tasks[taskId]
	if !exists {
		log.Printf("Задача #%d не найдена, но результат %f будет обработан", taskId, result)
	} else {
		log.Printf("Задача #%d найдена в системе", taskId)
	}

	// Сохраняем результат
	Manager.Results[taskId] = result

	// Удаляем задачу из списка обрабатываемых
	delete(Manager.ProcessingTasks, taskId)

	// Обработка зависимостей задач
	log.Printf("Начинаем обработку зависимостей для задачи #%d с результатом %f", taskId, result)
	processTaskDependencies(taskId, result)

	// Обновляем статусы всех выражений
	apiUpdateExpressions()

	return nil
}

func updateReadyTasks() {
	log.Printf("updateReadyTasks: начало обновления списка готовых задач")

	Manager.mu.Lock()
	defer Manager.mu.Unlock()

	// Очищаем текущий список готовых задач
	Manager.ReadyTasks = []models.Task{}

	// Проверяем все активные задачи
	for taskID, taskPtr := range Manager.Tasks {
		task := *taskPtr
		log.Printf("updateReadyTasks: проверка задачи #%d: %s %s %s", taskID, task.Arg1, task.Operation, task.Arg2)

		// Проверяем, не находится ли задача уже в обработке
		if Manager.ProcessingTasks[taskID] {
			log.Printf("updateReadyTasks: задача #%d уже в обработке, пропускаем", taskID)
			continue
		}

		// Проверяем готовность задачи
		if apiIsTaskReady(task) {
			log.Printf("updateReadyTasks: задача #%d готова к выполнению", taskID)
			Manager.ReadyTasks = append(Manager.ReadyTasks, task)
		} else {
			log.Printf("updateReadyTasks: задача #%d не готова к выполнению", taskID)

			// Проверяем, можем ли мы обновить аргументы задачи
			if strings.HasPrefix(task.Arg1, "result") {
				resultID := strings.TrimPrefix(task.Arg1, "result")
				if id, err := strconv.Atoi(resultID); err == nil {
					if result, exists := Manager.Results[id]; exists {
						log.Printf("updateReadyTasks: обновляем arg1 задачи #%d результатом задачи #%d: %f", taskID, id, result)
						task.Arg1 = strconv.FormatFloat(result, 'f', -1, 64)
					}
				}
			}

			if strings.HasPrefix(task.Arg2, "result") {
				resultID := strings.TrimPrefix(task.Arg2, "result")
				if id, err := strconv.Atoi(resultID); err == nil {
					if result, exists := Manager.Results[id]; exists {
						log.Printf("updateReadyTasks: обновляем arg2 задачи #%d результатом задачи #%d: %f", taskID, id, result)
						task.Arg2 = strconv.FormatFloat(result, 'f', -1, 64)
					}
				}
			}

			// Проверяем еще раз после обновления аргументов
			if apiIsTaskReady(task) {
				log.Printf("updateReadyTasks: задача #%d стала готова после обновления аргументов", taskID)
				*taskPtr = task
				Manager.ReadyTasks = append(Manager.ReadyTasks, task)
			} else {
				log.Printf("updateReadyTasks: задача #%d все еще не готова после обновления аргументов", taskID)
				*taskPtr = task
			}
		}
	}

	log.Printf("updateReadyTasks: обновление завершено, готово задач: %d", len(Manager.ReadyTasks))
}
