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
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Expression == "" {
		http.Error(w, "Invalid data", http.StatusUnprocessableEntity)
		return
	}

	exprID := "expr id:" + strconv.Itoa(exprCounter)
	exprCounter++
	mu.Lock()
	expressions[exprID] = &models.Expression{ID: exprID, Status: "pending"}
	mu.Unlock()

	go func() {
		log.Printf("Парсинг выражения: %s", input.Expression)
		taskList := ParseExpression(input.Expression)
		mu.Lock()
		log.Printf("Создание задач для выражения %s. Всего задач: %d", exprID, len(taskList))

		// Отображаем все созданные задачи
		log.Printf("Структура созданных задач для выражения %s:", exprID)
		for i, t := range taskList {
			log.Printf("Задача %d: ID=%d, %s %s %s, Готова: %v",
				i+1, t.ID, t.Arg1, t.Operation, t.Arg2, isTaskReady(t))
		}

		// Создаем карту зависимостей между задачами
		dependsOn := make(map[string][]int) // ключ: resultX, значение: список ID задач
		for _, t := range taskList {
			taskCounter++
			t.ID = taskCounter
			tasks[t.ID] = &t
			taskToExpr[t.ID] = exprID

			// Регистрируем зависимости для аргументов, которые являются результатами других задач
			if strings.HasPrefix(t.Arg1, "result") {
				dependsOn[t.Arg1] = append(dependsOn[t.Arg1], t.ID)
				log.Printf("Задача #%d зависит от результата %s", t.ID, t.Arg1)
			}
			if strings.HasPrefix(t.Arg2, "result") {
				dependsOn[t.Arg2] = append(dependsOn[t.Arg2], t.ID)
				log.Printf("Задача #%d зависит от результата %s", t.ID, t.Arg2)
			}

			if isTaskReady(t) {
				log.Printf("Готовая задача #%d (%s %s %s) для выражения %s добавлена в очередь",
					t.ID, t.Arg1, t.Operation, t.Arg2, exprID)
				readyTasks = append(readyTasks, t)
			} else {
				log.Printf("Задача #%d (%s %s %s) для выражения %s ожидает результатов других задач",
					t.ID, t.Arg1, t.Operation, t.Arg2, exprID)
			}
		}

		// Логируем зависимости
		log.Printf("Карта зависимостей для выражения %s:", exprID)
		for result, taskIDs := range dependsOn {
			taskIDsStr := make([]string, len(taskIDs))
			for i, id := range taskIDs {
				taskIDsStr[i] = strconv.Itoa(id)
			}
			log.Printf("Результат %s нужен для задач: %s",
				result, strings.Join(taskIDsStr, ", "))
		}

		mu.Unlock()
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
		mu.Lock()
		if len(readyTasks) == 0 {
			mu.Unlock()
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Ищем задачу, которая еще не в обработке
		taskIndex := -1
		for i, task := range readyTasks {
			if !processingTasks[task.ID] {
				taskIndex = i
				break
			}
		}

		// Если все задачи уже в обработке, возвращаем 404
		if taskIndex == -1 {
			mu.Unlock()
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Получаем задачу и удаляем ее из очереди
		task := readyTasks[taskIndex]

		// Если задача находится в середине очереди, перемещаем последнюю задачу на ее место
		// и усекаем очередь
		if taskIndex < len(readyTasks)-1 {
			readyTasks[taskIndex] = readyTasks[len(readyTasks)-1]
		}
		readyTasks = readyTasks[:len(readyTasks)-1]

		// Помечаем задачу как "в обработке"
		processingTasks[task.ID] = true

		mu.Unlock()

		log.Printf("Отправляем задачу #%d агенту: %s %s %s",
			task.ID, task.Arg1, task.Operation, task.Arg2)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]models.Task{"task": task})
		return
	}

	if r.Method == http.MethodPost {
		var result models.TaskResult
		if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
			log.Printf("TaskHandler: ошибка декодирования тела запроса: %v", err)
			http.Error(w, "Invalid data", http.StatusUnprocessableEntity)
			return
		}

		log.Printf("TaskHandler: получен результат для задачи #%d: %f", result.ID, result.Result)

		mu.Lock()
		defer mu.Unlock()

		log.Printf("TaskHandler: начало обработки результата для задачи #%d: %f", result.ID, result.Result)

		// Снимаем отметку "в обработке" с задачи
		delete(processingTasks, result.ID)

		// Проверяем, существует ли еще задача
		task, exists := tasks[result.ID]
		if !exists {
			// Проверяем, может быть результат для этой задачи уже был получен
			if _, resultExists := results[result.ID]; resultExists {
				log.Printf("TaskHandler: результат для задачи #%d уже был получен ранее. Значение: %f. Текущий результат: %f. Игнорируем дублирующий результат.",
					result.ID, results[result.ID], result.Result)
				w.WriteHeader(http.StatusOK)
				return
			}

			log.Printf("TaskHandler: задача #%d не найдена в списке активных задач", result.ID)
			// Сохраняем результат даже для неактивной задачи
			results[result.ID] = result.Result
			log.Printf("TaskHandler: результат для неактивной задачи #%d сохранен: %f", result.ID, result.Result)

			// Получаем ID выражения, если оно есть
			exprID, exprExists := taskToExpr[result.ID]
			if exprExists {
				log.Printf("TaskHandler: неактивная задача #%d принадлежит выражению %s, проверяем зависимости", result.ID, exprID)

				// Обрабатываем зависимости даже для неактивной задачи
				processTaskDependencies(result.ID, result.Result)

				// Обновляем готовые задачи для выражения
				updateReadyTasks()

				// Проверяем статусы выражений
				updateExpressions()
			} else {
				log.Printf("TaskHandler: для неактивной задачи #%d не найдено связанное выражение", result.ID)
			}

			w.WriteHeader(http.StatusOK)
			return
		}

		exprID := taskToExpr[result.ID]

		log.Printf("TaskHandler: задача #%d (выражение %s): %s %s %s -> %f",
			result.ID, exprID, task.Arg1, task.Operation, task.Arg2, result.Result)

		// Сохраняем результат
		results[result.ID] = result.Result

		// Находим ID выражения для этой задачи
		log.Printf("TaskHandler: задача #%d принадлежит выражению %s", result.ID, exprID)

		// Удаляем выполненную задачу из активных
		delete(tasks, result.ID)

		// Обрабатываем зависимости - это ключевой метод
		processTaskDependencies(result.ID, result.Result)

		// Обновляем готовые задачи - новый метод для проверки задач в очереди готовых
		updateReadyTasks()

		// Проверяем, остались ли ещё задачи для этого выражения
		tasksRemain := false
		for taskID, id := range taskToExpr {
			if id == exprID {
				if _, ok := tasks[taskID]; ok {
					tasksRemain = true
					log.Printf("TaskHandler: для выражения %s осталась невыполненная задача #%d", exprID, taskID)
					break
				}
			}
		}

		if !tasksRemain {
			log.Printf("TaskHandler: для выражения %s не осталось задач, проверяем результат", exprID)
		}

		// Проверяем статусы выражений после обработки результата
		updateExpressions()

		log.Printf("TaskHandler: завершена обработка результата для задачи #%d: %f", result.ID, result.Result)
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
		if isTaskReady(task) {
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
func isTaskReady(t models.Task) bool {
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

// Проверяет, находится ли задача в очереди готовых задач
func isTaskInQueue(task models.Task, queue []models.Task) bool {
	for _, t := range queue {
		if t.ID == task.ID {
			return true
		}
	}
	return false
}

func updateExpressions() {
	log.Println("updateExpressions: проверка статусов выражений")

	// Итерируем по всем выражениям
	for exprID, expr := range expressions {
		// Пропускаем уже выполненные выражения
		if expr.Status == "completed" {
			log.Printf("updateExpressions: выражение %s уже выполнено, пропускаем", exprID)
			continue
		}

		log.Printf("updateExpressions: проверка выражения %s (статус %s)", exprID, expr.Status)

		// Собираем все задачи для этого выражения
		exprTaskIDs := []int{}
		for taskID, id := range taskToExpr {
			if id == exprID {
				exprTaskIDs = append(exprTaskIDs, taskID)
			}
		}
		log.Printf("updateExpressions: для выражения %s найдено %d связанных задач", exprID, len(exprTaskIDs))

		// Если для выражения не найдено задач, возможно парсинг не прошел
		if len(exprTaskIDs) == 0 {
			log.Printf("updateExpressions: для выражения %s не найдено задач, возможно проблема с парсингом", exprID)
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
			if _, exists := tasks[taskID]; exists {
				activeTaskIDs = append(activeTaskIDs, taskID)
				log.Printf("updateExpressions: выражение %s - задача #%d в списке активных", exprID, taskID)
			}

			// Проверяем, находится ли задача в процессе выполнения
			if processingTasks[taskID] {
				processingTaskIDs = append(processingTaskIDs, taskID)
				log.Printf("updateExpressions: выражение %s - задача #%d в процессе выполнения", exprID, taskID)
			}

			// Проверяем, имеет ли задача сохраненный результат
			if _, hasResult := results[taskID]; hasResult {
				completedTaskIDs = append(completedTaskIDs, taskID)
				log.Printf("updateExpressions: выражение %s - задача #%d имеет сохраненный результат", exprID, taskID)
			}
		}

		log.Printf("updateExpressions: выражение %s - статистика: %d активных, %d в обработке, %d выполнено",
			exprID, len(activeTaskIDs), len(processingTaskIDs), len(completedTaskIDs))

		// Проверяем, есть ли задачи в очереди готовых задач
		readyTasksCount := 0
		for _, task := range readyTasks {
			if taskToExpr[task.ID] == exprID {
				readyTasksCount++
				log.Printf("updateExpressions: выражение %s - задача #%d находится в очереди готовых задач",
					exprID, task.ID)
			}
		}

		log.Printf("updateExpressions: выражение %s имеет %d задач в очереди готовых", exprID, readyTasksCount)

		// Если нет активных задач, нет задач в обработке, нет задач в очереди готовых,
		// но есть выполненные задачи, то выражение завершено
		if len(activeTaskIDs) == 0 && len(processingTaskIDs) == 0 && readyTasksCount == 0 && len(completedTaskIDs) > 0 {
			log.Printf("updateExpressions: для выражения %s нет активных задач и задач в обработке, есть выполненные задачи",
				exprID)

			// Находим задачу с самым высоким ID - это должна быть последняя операция в выражении
			var lastTaskID int
			for _, taskID := range completedTaskIDs {
				if taskID > lastTaskID {
					lastTaskID = taskID
				}
			}

			log.Printf("updateExpressions: задача с самым высоким ID для выражения %s - #%d", exprID, lastTaskID)

			// Получаем результат последней задачи
			lastResult, hasResult := results[lastTaskID]
			if hasResult {
				log.Printf("updateExpressions: выражение %s завершено, результат %f от задачи #%d",
					exprID, lastResult, lastTaskID)
				expr.Status = "completed"
				expr.Result = lastResult
			} else {
				log.Printf("updateExpressions: не найден результат для задачи #%d выражения %s", lastTaskID, exprID)

				// Если не нашли результат последней задачи, ищем любой последний результат
				if len(completedTaskIDs) > 0 {
					// Берем последний добавленный результат
					for _, taskID := range completedTaskIDs {
						if res, ok := results[taskID]; ok {
							log.Printf("updateExpressions: используем результат %f от задачи #%d для выражения %s",
								res, taskID, exprID)
							expr.Status = "completed"
							expr.Result = res
							break
						}
					}
				}
			}
		}
	}
}

// Проверяет, находится ли задача в процессе обработки
func isTaskProcessing(taskID int) bool {
	return processingTasks[taskID]
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
	updateExpressions()

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
		if isTaskReady(task) {
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
			if isTaskReady(task) {
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
