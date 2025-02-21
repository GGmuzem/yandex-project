package orchestrator

import (
    "os"
    "strconv"
    "strings"
    "github.com/GGmuzem/yandex-project/pkg/models"
)

func ParseExpression(expr string) []models.Task {
    tokens := strings.Fields(expr)
    var numbers []string
    var operators []string

    for _, token := range tokens {
        if isOperator(token) {
            for len(operators) > 0 && precedence(operators[len(operators)-1]) >= precedence(token) {
                processOperator(&numbers, &operators)
            }
            operators = append(operators, token)
        } else {
            numbers = append(numbers, token)
        }
    }
    for len(operators) > 0 {
        processOperator(&numbers, &operators)
    }

    var tasks []models.Task
    for i := 0; i < len(operators); i++ {
        tasks = append(tasks, models.Task{
            Arg1:          numbers[i],
            Arg2:          numbers[i+1],
            Operation:     operators[i],
            OperationTime: getOperationTime(operators[i]),
        })
    }
    return tasks
}

func processOperator(numbers *[]string, operators *[]string) {
    op := (*operators)[len(*operators)-1]
    *operators = (*operators)[:len(*operators)-1]
    b := (*numbers)[len(*numbers)-1]
    a := (*numbers)[len(*numbers)-2]
    *numbers = (*numbers)[:len(*numbers)-2]
    *numbers = append(*numbers, a, b)
    *operators = append(*operators, op) // Упрощённо, для очереди задач
}

func isOperator(token string) bool {
    return token == "+" || token == "-" || token == "*" || token == "/"
}

func precedence(op string) int {
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