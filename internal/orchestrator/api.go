package orchestrator

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/GGmuzem/yandex-project/pkg/models"
	"github.com/GGmuzem/yandex-project/internal/database"
)

// Статусы задач
const (
	TaskStatusNew        = 0 // Новая задача
	TaskStatusReady      = 1 // Готова к выполнению
	TaskStatusInProgress = 2 // В процессе выполнения
	TaskStatusCompleted  = 3 // Выполнена
)

// Используем глобальный Manager из manager.go

// DB is the database instance for persisting results and expression statuses
var DB database.Database

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
	
	// Обновляем статусы выражений и список готовых задач
	log.Printf("Вызов apiUpdateExpressions после добавления задач")
	apiUpdateExpressions()

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
	log.Printf("GetExpressionHandler: Запрошено выражение %s", id)

	// Получаем выражение из менеджера
	Manager.mu.Lock()
	expr, exists := Manager.Expressions[id]
	Manager.mu.Unlock()

	if !exists {
		log.Printf("GetExpressionHandler: Выражение %s не найдено", id)
		http.Error(w, "Expression not found", http.StatusNotFound)
		return
	}

	// Проверяем, есть ли результаты в БД
	if expr.Status == "completed" && DB != nil {
		// Получаем последний результат для этого выражения из БД
		results, err := DB.GetResultsByExprID(id)
		if err != nil {
			log.Printf("GetExpressionHandler: Ошибка при получении результатов из БД: %v", err)
		} else if len(results) > 0 {
			// Находим последний результат (с наибольшим ID)
			lastTaskID := 0
			lastResult := 0.0
			for taskID, result := range results {
				if taskID > lastTaskID {
					lastTaskID = taskID
					lastResult = result
				}
			}
			
			log.Printf("GetExpressionHandler: Найден последний результат для выражения %s: %f (задача #%d)", id, lastResult, lastTaskID)
			
			// Обновляем результат выражения
			expr.Result = lastResult
			
			// Обновляем также в менеджере
			Manager.mu.Lock()
			if e, ok := Manager.Expressions[id]; ok {
				e.Result = lastResult
			}
			Manager.mu.Unlock()
		}
	}

	log.Printf("GetExpressionHandler: Отправляем выражение %s со статусом %s и результатом %f", id, expr.Status, expr.Result)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]models.Expression{"expression": *expr})
}

func TaskHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("TaskHandler: incoming", r.Method)
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
			
			// Находим ID выражения для этой задачи
			exprID := ""
			for taskID, id := range Manager.TaskToExpr {
				if taskID == result.ID {
					exprID = id
					break
				}
			}
			
			if exprID != "" {
				// Сохраняем результат в память
				Manager.mu.Lock()
				Manager.Results[result.ID] = result.Result
				log.Printf("TaskHandler POST: результат задачи #%d сохранен в памяти: %f", result.ID, result.Result)
				Manager.mu.Unlock()
				
				// Проверяем, остались ли задачи для этого выражения
				tasksRemain := false
				Manager.mu.Lock()
				for taskID, id := range Manager.TaskToExpr {
					if id == exprID {
						if _, ok := Manager.Tasks[taskID]; ok {
							tasksRemain = true
							log.Printf("TaskHandler POST: Для выражения %s осталась невыполненная задача #%d", exprID, taskID)
							break
						}
					}
				}
				Manager.mu.Unlock()
				
				// Если задач больше нет, обновляем результат выражения в БД
				if !tasksRemain {
					log.Printf("TaskHandler POST: Для выражения %s не осталось задач, определяем финальный результат", exprID)
					
					// Находим задачу с самым большим ID для этого выражения
					finalTaskID := 0
					finalResult := 0.0
					
					// Собираем все задачи с результатами для этого выражения
					type taskWithResult struct {
						id     int
						result float64
					}
					var tasksWithResults []taskWithResult
					
					Manager.mu.Lock()
					for taskID, id := range Manager.TaskToExpr {
						if id != exprID {
							continue
						}
						
						// Проверяем, есть ли результат для этой задачи
						taskResult, resultExists := Manager.Results[taskID]
						if !resultExists {
							log.Printf("TaskHandler POST: Задача #%d не имеет результата", taskID)
							continue
						}
						
						// Добавляем задачу в список
						tasksWithResults = append(tasksWithResults, taskWithResult{id: taskID, result: taskResult})
						log.Printf("TaskHandler POST: Задача #%d имеет результат %f", taskID, taskResult)
					}
					Manager.mu.Unlock()
					
					// Сортируем задачи по ID в порядке убывания (самый большой ID будет первым)
					sort.Slice(tasksWithResults, func(i, j int) bool {
						return tasksWithResults[i].id > tasksWithResults[j].id
					})
					
					// Берем задачу с самым большим ID
					if len(tasksWithResults) > 0 {
						finalTaskID = tasksWithResults[0].id
						finalResult = tasksWithResults[0].result
						log.Printf("TaskHandler POST: Выбрана задача с самым большим ID #%d с результатом %f для выражения %s", finalTaskID, finalResult, exprID)
						
						// Обновляем выражение в памяти
						Manager.mu.Lock()
						if expr, ok := Manager.Expressions[exprID]; ok {
							expr.Status = "completed"
							expr.Result = finalResult
							log.Printf("TaskHandler POST: Обновлено выражение %s в памяти: статус=completed, результат=%f", exprID, finalResult)
						} else {
							log.Printf("TaskHandler POST: Выражение %s не найдено в памяти", exprID)
						}
						Manager.mu.Unlock()
						
						// Обновляем выражение в БД
						if err := DB.UpdateExpressionStatus(exprID, "completed", finalResult); err != nil {
							log.Printf("TaskHandler POST: Ошибка при обновлении выражения %s в БД: %v", exprID, err)
						} else {
							log.Printf("TaskHandler POST: Успешно обновлено выражение %s в БД: статус=completed, результат=%f", exprID, finalResult)
						}
					} else {
						log.Printf("TaskHandler POST: Не найден финальный результат для выражения %s", exprID)
					}
				} else {
					log.Printf("TaskHandler POST: результат задачи #%d сохранен в БД", result.ID)
				}
				
					// Затем обновляем статусы выражений в памяти
				log.Printf("TaskHandler POST: Вызываем apiUpdateExpressions для обновления статусов выражений")
				apiUpdateExpressions()
				
				// Обновляем список готовых задач
				Manager.updateReadyTasksList()
				
				// Проверяем состояние менеджера после обновления
				log.Printf("TaskHandler POST: Всего задач в системе: %d", len(Manager.Tasks))
				log.Printf("TaskHandler POST: Всего результатов в системе: %d", len(Manager.Results))
				log.Printf("TaskHandler POST: Всего готовых задач: %d", len(Manager.ReadyTasks))
			}
			
			// Respond with expression statuses
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string][]models.Expression{"expressions": Manager.GetAllExpressions()})
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
		if IsTaskReady(task) {
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
	Manager.updateReadyTasksList()
	// Вызываем глобальную функцию обновления статусов выражений
	UpdateExpressions()
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

// Функция updateReadyTasks удалена, теперь используется метод Manager.updateReadyTasksList() из manager.go
