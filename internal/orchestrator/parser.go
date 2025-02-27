package orchestrator

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/GGmuzem/yandex-project/pkg/models"
)

// ParseExpression разбирает строку с арифметическим выражением и создает список задач
func ParseExpression(expr string) []models.Task {
	// Подробное логирование входного выражения
	log.Printf("Начало парсинга выражения: '%s'", expr)

	// Предварительная обработка выражения
	expr = strings.ReplaceAll(expr, "(", " ( ")
	expr = strings.ReplaceAll(expr, ")", " ) ")
	expr = strings.ReplaceAll(expr, "+", " + ")
	expr = strings.ReplaceAll(expr, "-", " - ")
	expr = strings.ReplaceAll(expr, "*", " * ")
	expr = strings.ReplaceAll(expr, "/", " / ")

	// Логируем обработанное выражение
	log.Printf("После форматирования: '%s'", expr)

	tokens := strings.Fields(expr)
	log.Printf("Токены: %v", tokens)

	output := []string{}    // Выходная очередь (постфиксная запись)
	operators := []string{} // Стек операторов

	// Алгоритм сортировочной станции (Shunting yard)
	for i, token := range tokens {
		log.Printf("Обработка токена %d: '%s'", i, token)
		if token == "(" {
			log.Printf("Добавление '(' в стек операторов")
			operators = append(operators, token)
		} else if token == ")" {
			log.Printf("Найдена закрывающая скобка, извлекаем операторы до открывающей скобки")
			for len(operators) > 0 && operators[len(operators)-1] != "(" {
				popped := operators[len(operators)-1]
				output = append(output, popped)
				operators = operators[:len(operators)-1]
				log.Printf("  Извлекаем оператор '%s' в выходную очередь", popped)
			}
			if len(operators) > 0 && operators[len(operators)-1] == "(" {
				log.Printf("  Удаляем открывающую скобку из стека")
				operators = operators[:len(operators)-1] // Удаляем левую скобку
			} else {
				log.Printf("  ВНИМАНИЕ: Не найдена соответствующая открывающая скобка")
			}
		} else if isOperator(token) {
			log.Printf("Найден оператор '%s'", token)
			for len(operators) > 0 && precedence(operators[len(operators)-1]) >= precedence(token) {
				popped := operators[len(operators)-1]
				output = append(output, popped)
				operators = operators[:len(operators)-1]
				log.Printf("  Извлекаем оператор '%s' с более высоким приоритетом", popped)
			}
			log.Printf("  Добавляем оператор '%s' в стек", token)
			operators = append(operators, token)
		} else {
			// Числа добавляем прямо в выходную очередь
			log.Printf("Добавление числа '%s' в выходную очередь", token)
			output = append(output, token)
		}
		log.Printf("Текущее состояние: стек операторов=%v, выходная очередь=%v", operators, output)
	}

	// Перемещаем оставшиеся операторы в выходную очередь
	log.Printf("Извлечение оставшихся операторов из стека")
	for len(operators) > 0 {
		popped := operators[len(operators)-1]
		output = append(output, popped)
		operators = operators[:len(operators)-1]
		log.Printf("  Извлекаем оператор '%s' в выходную очередь", popped)
	}

	log.Printf("Итоговая постфиксная запись: %v", output)

	// Преобразуем постфиксную запись в список задач
	tasks := createTasksFromPostfix(output)
	log.Printf("Создано %d задач для выражения", len(tasks))
	return tasks
}

// createTasksFromPostfix создает задачи из массива токенов в постфиксной записи
func createTasksFromPostfix(tokens []string) []models.Task {
	log.Printf("Начало создания задач из постфиксной записи. Токены: %v", tokens)

	// Стек для результатов операций
	stack := []string{}
	var tasks []models.Task

	log.Printf("Инициализирован пустой стек и список задач")

	for i, token := range tokens {
		log.Printf("Обработка токена %d: '%s'", i, token)

		if isOperator(token) {
			log.Printf("Токен '%s' - это оператор", token)

			if len(stack) < 2 {
				log.Printf("ОШИБКА: Недостаточно операндов для операции %s. Текущий стек: %v", token, stack)
				continue
			}

			// Получаем два последних операнда со стека
			operand2 := stack[len(stack)-1]
			operand1 := stack[len(stack)-2]
			stack = stack[:len(stack)-2] // Удаляем операнды

			log.Printf("Получены операнды: arg1='%s', arg2='%s'", operand1, operand2)

			// Создаем новую задачу
			taskID := len(tasks) + 1 // ID задачи (начиная с 1)
			resultID := fmt.Sprintf("result%d", taskID)

			log.Printf("Создание задачи #%d: %s %s %s -> %s",
				taskID, operand1, token, operand2, resultID)

			task := models.Task{
				ID:            taskID,
				Arg1:          operand1,
				Arg2:          operand2,
				Operation:     token,
				OperationTime: getOperationTime(token),
			}

			// Дополнительная проверка и преобразование аргументов задачи
			// Попытаемся преобразовать операнды в числа, если они не ссылки на другие результаты
			if !strings.HasPrefix(operand1, "result") {
				_, err := strconv.ParseFloat(operand1, 64)
				if err != nil {
					log.Printf("ПРЕДУПРЕЖДЕНИЕ: Операнд '%s' не является ни числом, ни ссылкой на результат", operand1)
				}
			}
			if !strings.HasPrefix(operand2, "result") {
				_, err := strconv.ParseFloat(operand2, 64)
				if err != nil {
					log.Printf("ПРЕДУПРЕЖДЕНИЕ: Операнд '%s' не является ни числом, ни ссылкой на результат", operand2)
				}
			}

			tasks = append(tasks, task)
			log.Printf("Задача #%d добавлена в список задач", taskID)

			// Результат этой операции становится доступным для последующих операций
			stack = append(stack, resultID)
			log.Printf("Результат '%s' добавлен на стек. Текущий стек: %v", resultID, stack)
		} else {
			// Если число - просто добавляем на стек
			stack = append(stack, token)
			log.Printf("Число '%s' добавлено на стек. Текущий стек: %v", token, stack)
		}
	}

	// Проверяем, что в стеке остался только один элемент
	if len(stack) != 1 {
		log.Printf("ПРЕДУПРЕЖДЕНИЕ: После обработки всех токенов в стеке осталось %d элементов: %v", len(stack), stack)
	} else {
		log.Printf("Успешное завершение. В стеке остался один элемент: %s", stack[0])
	}

	// Проверяем зависимости задач
	log.Printf("Проверка зависимостей между задачами:")
	dependencyMap := make(map[string][]int) // Карта зависимостей: ключ - resultX, значение - список ID задач

	for i, task := range tasks {
		taskID := i + 1
		log.Printf("Задача #%d (%s %s %s):", taskID, task.Arg1, task.Operation, task.Arg2)

		// Проверяем зависимость по Arg1
		if strings.HasPrefix(task.Arg1, "result") {
			sourceTaskID, err := strconv.Atoi(strings.TrimPrefix(task.Arg1, "result"))
			if err == nil {
				log.Printf("  - зависит от результата задачи #%d по аргументу 1", sourceTaskID)
				dependencyMap[task.Arg1] = append(dependencyMap[task.Arg1], taskID)
			} else {
				log.Printf("  - ОШИБКА при разборе ID задачи из %s", task.Arg1)
			}
		} else {
			log.Printf("  - аргумент 1 (%s) не зависит от других задач", task.Arg1)
		}

		// Проверяем зависимость по Arg2
		if strings.HasPrefix(task.Arg2, "result") {
			sourceTaskID, err := strconv.Atoi(strings.TrimPrefix(task.Arg2, "result"))
			if err == nil {
				log.Printf("  - зависит от результата задачи #%d по аргументу 2", sourceTaskID)
				dependencyMap[task.Arg2] = append(dependencyMap[task.Arg2], taskID)
			} else {
				log.Printf("  - ОШИБКА при разборе ID задачи из %s", task.Arg2)
			}
		} else {
			log.Printf("  - аргумент 2 (%s) не зависит от других задач", task.Arg2)
		}
	}

	// Логируем зависимости
	for resultID, dependentTasks := range dependencyMap {
		taskIDs := make([]string, len(dependentTasks))
		for i, id := range dependentTasks {
			taskIDs[i] = strconv.Itoa(id)
		}
		log.Printf("Результат %s нужен для задач: %s", resultID, strings.Join(taskIDs, ", "))
	}

	// Проверка на циклические зависимости
	for i, task := range tasks {
		taskID := i + 1
		log.Printf("Проверка циклических зависимостей для задачи #%d...", taskID)

		// Если оба аргумента - результаты других задач
		if strings.HasPrefix(task.Arg1, "result") && strings.HasPrefix(task.Arg2, "result") {
			sourceTaskID1, err1 := strconv.Atoi(strings.TrimPrefix(task.Arg1, "result"))
			sourceTaskID2, err2 := strconv.Atoi(strings.TrimPrefix(task.Arg2, "result"))

			if err1 == nil && err2 == nil {
				log.Printf("  Задача #%d зависит от результатов задач #%d и #%d", taskID, sourceTaskID1, sourceTaskID2)
				// Проверяем, что эти задачи существуют и имеют правильные ID
				if sourceTaskID1 > len(tasks) || sourceTaskID2 > len(tasks) {
					log.Printf("  ПРЕДУПРЕЖДЕНИЕ: Задача #%d ссылается на несуществующие задачи!", taskID)
				}
			}
		}
	}

	log.Printf("Итоговые задачи: %+v", tasks)
	return tasks
}

func isOperator(token string) bool {
	return token == "+" || token == "-" || token == "*" || token == "/"
}

func precedence(op string) int {
	if op == "(" || op == ")" {
		return 0
	}
	if op == "+" || op == "-" {
		return 1
	}
	if op == "*" || op == "/" {
		return 2
	}
	return 0
}

func getOperationTime(op string) int {
	switch op {
	case "+":
		return getEnvInt("TIME_ADDITION_MS", 100)
	case "-":
		return getEnvInt("TIME_SUBTRACTION_MS", 100)
	case "*":
		return getEnvInt("TIME_MULTIPLICATIONS_MS", 200)
	case "/":
		return getEnvInt("TIME_DIVISIONS_MS", 200)
	}
	return 100
}

func getEnvInt(key string, defaultVal int) int {
	if val, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return val
	}
	return defaultVal
}
