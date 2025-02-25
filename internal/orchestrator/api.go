package orchestrator

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/GGmuzem/yandex-project/pkg/models"
)

var (
	expressions = make(map[string]*models.Expression)
	tasks       = make(map[int]*models.Task)
	results     = make(map[int]float64)
	readyTasks  = []models.Task{}
	taskToExpr  = make(map[int]string)
	mu          sync.Mutex
	taskCounter int
	exprCounter int
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
		task := readyTasks[0]
		readyTasks = readyTasks[1:]
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]models.Task{"task": task})
		return
	}

	if r.Method == http.MethodPost {
		var result models.TaskResult
		if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
			http.Error(w, "Invalid data", http.StatusUnprocessableEntity)
			return
		}

		mu.Lock()
		if _, exists := tasks[result.ID]; !exists {
			mu.Unlock()
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}

		log.Printf("Получен результат для задачи #%d: %f", result.ID, result.Result)

		// Получаем информацию о задаче перед её удалением
		task := tasks[result.ID]
		exprID := taskToExpr[result.ID]

		log.Printf("Задача #%d (выражение %s): %s %s %s -> %f",
			result.ID, exprID, task.Arg1, task.Operation, task.Arg2, result.Result)

		// Сохраняем результат
		results[result.ID] = result.Result

		// Находим ID выражения для этой задачи
		log.Printf("Задача #%d принадлежит выражению %s", result.ID, exprID)

		// Удаляем выполненную задачу
		delete(tasks, result.ID)

		// Обрабатываем зависимости - это ключевой метод
		processTaskDependencies(result.ID, result.Result)

		// Проверяем, остались ли ещё задачи для этого выражения
		tasksRemain := false
		for taskID, id := range taskToExpr {
			if id == exprID {
				if _, ok := tasks[taskID]; ok {
					tasksRemain = true
					break
				}
			}
		}

		if !tasksRemain {
			log.Printf("Для выражения %s не осталось задач, проверяем результат", exprID)
		}

		// Проверяем статусы выражений после обработки результата
		updateExpressions()

		mu.Unlock()

		w.WriteHeader(http.StatusOK)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// Функция для рекурсивной обработки зависимостей задач
func processTaskDependencies(taskID int, taskResult float64) {
	resultStr := "result" + strconv.Itoa(taskID)
	log.Printf("Обработка зависимостей для задачи #%d с результатом %f (resultStr=%s)", taskID, taskResult, resultStr)

	// Создаем отдельную карту для отслеживания обработанных задач
	processed := make(map[int]bool)

	// Считаем, сколько задач было в готовой очереди до обработки зависимостей
	initialReadyCount := len(readyTasks)
	log.Printf("Начальное состояние очереди: %d задач готовы к выполнению", initialReadyCount)

	// Устанавливаем максимальное количество итераций
	maxIterations := 20

	// Отслеживаем, были ли изменения в последней итерации
	changesDetected := true

	for iterations := 0; iterations < maxIterations && changesDetected; iterations++ {
		log.Printf("Итерация %d обработки зависимостей для результата %s", iterations, resultStr)

		// Для каждой итерации создаем карту уже обработанных задач
		processedTasks := make(map[int]bool)
		changesDetected = false

		// Проходим по всем задачам и обновляем их, если они зависят от результата текущей задачи
		for id, taskPtr := range tasks {
			// Пропускаем уже обработанные задачи в предыдущих итерациях
			if processed[id] {
				continue
			}

			task := *taskPtr

			// Пропускаем задачи, которые уже готовы к выполнению
			if isTaskReady(task) {
				continue
			}

			taskChanged := false

			// Обновляем аргументы задачи, если они зависят от результата выполненной задачи
			if task.Arg1 == resultStr {
				taskChanged = true
				oldArg1 := task.Arg1
				task.Arg1 = fmt.Sprintf("%f", taskResult)
				log.Printf("Обновление задачи #%d: arg1 изменен с %s на %s", id, oldArg1, task.Arg1)
			}

			if task.Arg2 == resultStr {
				taskChanged = true
				oldArg2 := task.Arg2
				task.Arg2 = fmt.Sprintf("%f", taskResult)
				log.Printf("Обновление задачи #%d: arg2 изменен с %s на %s", id, oldArg2, task.Arg2)
			}

			// Если задача была изменена
			if taskChanged {
				changesDetected = true
				tasks[id] = &task
				log.Printf("Задача #%d обновлена: %s %s %s", id, task.Arg1, task.Operation, task.Arg2)

				// Помечаем задачу как обработанную для этой итерации
				processedTasks[id] = true

				// Если задача готова к выполнению, добавляем её в очередь
				if isTaskReady(task) {
					log.Printf("Задача #%d готова к выполнению и добавлена в очередь", id)
					readyTasks = append(readyTasks, task)
				}
			}
		}

		// Добавляем обработанные задачи из текущей итерации в общий список
		for id := range processedTasks {
			processed[id] = true
		}

		// Если в этой итерации не было изменений, завершаем цикл
		if !changesDetected {
			log.Printf("Итерация %d: изменений не обнаружено, завершаем цикл", iterations)
			break
		}
	}

	// Если количество задач в очереди не увеличилось, проверяем все зависимости вручную
	if len(readyTasks) == initialReadyCount {
		log.Printf("После обработки зависимостей количество готовых задач не изменилось. Проверяем все задачи вручную")

		// Проверяем каждую задачу и добавляем в очередь, если она готова
		for id, taskPtr := range tasks {
			task := *taskPtr
			if isTaskReady(task) && !isTaskInQueue(task, readyTasks) {
				log.Printf("Задача #%d готова к выполнению, но не была добавлена в очередь. Добавляем её", id)
				readyTasks = append(readyTasks, task)
			}
		}
	}

	// Обновляем статусы выражений после обработки зависимостей
	updateExpressions()
}

// Проверяет, находится ли задача с указанным ID в очереди готовых задач
func isTaskReady(t models.Task) bool {
	_, err1 := strconv.ParseFloat(t.Arg1, 64)
	_, err2 := strconv.ParseFloat(t.Arg2, 64)
	return err1 == nil && err2 == nil
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
	for exprID, expr := range expressions {
		if expr.Status == "completed" {
			continue
		}

		log.Printf("updateExpressions: проверка выражения %s (статус %s)", exprID, expr.Status)

		if exprID != expr.ID {
			log.Printf("updateExpressions: ВНИМАНИЕ! exprID из карты (%s) не совпадает с expr.ID (%s)", exprID, expr.ID)
			exprID = expr.ID
		}

		// Находим все задачи, связанные с этим выражением
		exprTasks := make(map[int]bool)
		for taskID, id := range taskToExpr {
			if id == exprID {
				exprTasks[taskID] = true
			}
		}

		// Проверяем, остались ли незавершенные задачи для этого выражения
		allTasksDone := true
		for taskID := range tasks {
			if taskToExpr[taskID] == exprID {
				allTasksDone = false
				log.Printf("updateExpressions: выражение %s - есть незавершенные задачи: #%d", exprID, taskID)
				break
			}
		}

		if allTasksDone {
			log.Printf("updateExpressions: все задачи для выражения %s выполнены", exprID)

			// Находим результат с самым большим ID задачи (последний выполненный результат)
			var lastResult float64
			var lastTaskID int
			foundResult := false

			for taskID, result := range results {
				if _, ok := exprTasks[taskID]; ok {
					// Если это задача из нашего выражения
					if !foundResult || taskID > lastTaskID {
						lastTaskID = taskID
						lastResult = result
						foundResult = true
						log.Printf("updateExpressions: найден результат %f для задачи #%d выражения %s",
							result, taskID, exprID)
					}
				}
			}

			if foundResult {
				log.Printf("updateExpressions: обновляем статус выражения %s на completed, результат %f", exprID, lastResult)
				expr.Status = "completed"
				expr.Result = lastResult
			} else {
				log.Printf("updateExpressions: результат для выражения %s не найден, проверяем TaskToExpr", exprID)

				// Проверяем, были ли вообще созданы задачи для этого выражения
				foundTasks := false
				for _, expID := range taskToExpr {
					if expID == exprID {
						foundTasks = true
						log.Printf("updateExpressions: найдены задачи для выражения %s", exprID)
						break
					}
				}

				if !foundTasks {
					log.Printf("updateExpressions: для выражения %s не было создано задач", exprID)
				}
			}
		}
	}
}
