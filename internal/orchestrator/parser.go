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
	// Предварительная обработка выражения
	expr = strings.ReplaceAll(expr, "(", " ( ")
	expr = strings.ReplaceAll(expr, ")", " ) ")
	expr = strings.ReplaceAll(expr, "+", " + ")
	expr = strings.ReplaceAll(expr, "-", " - ")
	expr = strings.ReplaceAll(expr, "*", " * ")
	expr = strings.ReplaceAll(expr, "/", " / ")

	tokens := strings.Fields(expr)
	output := []string{}    // Выходная очередь (постфиксная запись)
	operators := []string{} // Стек операторов

	// Алгоритм сортировочной станции (Shunting yard)
	for _, token := range tokens {
		if token == "(" {
			operators = append(operators, token)
		} else if token == ")" {
			for len(operators) > 0 && operators[len(operators)-1] != "(" {
				output = append(output, operators[len(operators)-1])
				operators = operators[:len(operators)-1]
			}
			if len(operators) > 0 && operators[len(operators)-1] == "(" {
				operators = operators[:len(operators)-1] // Удаляем левую скобку
			}
		} else if isOperator(token) {
			for len(operators) > 0 && precedence(operators[len(operators)-1]) >= precedence(token) {
				output = append(output, operators[len(operators)-1])
				operators = operators[:len(operators)-1]
			}
			operators = append(operators, token)
		} else {
			// Числа добавляем прямо в выходную очередь
			output = append(output, token)
		}
	}

	// Перемещаем оставшиеся операторы в выходную очередь
	for len(operators) > 0 {
		output = append(output, operators[len(operators)-1])
		operators = operators[:len(operators)-1]
	}

	// Преобразуем постфиксную запись в список задач
	return createTasksFromPostfix(output)
}

// createTasksFromPostfix создает задачи из массива токенов в постфиксной записи
func createTasksFromPostfix(tokens []string) []models.Task {
	log.Printf("Парсинг выражения, токены: %v", tokens)

	// Стек для результатов операций
	stack := []string{}
	var tasks []models.Task

	for _, token := range tokens {
		if isOperator(token) {
			if len(stack) < 2 {
				log.Printf("Ошибка: недостаточно операндов для операции %s", token)
				continue
			}

			// Получаем два последних операнда со стека
			operand2 := stack[len(stack)-1]
			operand1 := stack[len(stack)-2]
			stack = stack[:len(stack)-2] // Удаляем операнды

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

			tasks = append(tasks, task)

			// Результат этой операции становится доступным для последующих операций
			stack = append(stack, resultID)
		} else {
			// Если число - просто добавляем на стек
			stack = append(stack, token)
			log.Printf("Добавление числа в стек: %s", token)
		}
	}

	// Проверяем зависимости задач
	for i := range tasks {
		log.Printf("Проверка зависимостей для задачи #%d", i+1)

		// Пример: Если Arg1 или Arg2 - это "resultX", то задача зависит от результата задачи X
		if strings.HasPrefix(tasks[i].Arg1, "result") {
			log.Printf("Задача #%d зависит от результата задачи по arg1: %s",
				i+1, tasks[i].Arg1)
		}

		if strings.HasPrefix(tasks[i].Arg2, "result") {
			log.Printf("Задача #%d зависит от результата задачи по arg2: %s",
				i+1, tasks[i].Arg2)
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
