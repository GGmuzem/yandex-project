package calculate

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"strconv"
)

// CalcError определяет ошибку обработки выражения
type CalcError struct {
	Message string
}

func (e *CalcError) Error() string {
	return e.Message
}

// Evaluate принимает строковое выражение и возвращает результат
func Evaluate(expression string) (string, error) {
	// Проверяем наличие недопустимых символов
	if !isValidExpression(expression) {
		return "", &CalcError{Message: "Invalid expression"}
	}

	// Вычисляем выражение с использованием Go parser
	fs := token.NewFileSet()
	node, err := parser.ParseExpr(expression)
	if err != nil {
		return "", &CalcError{Message: "Expression is not valid"}
	}

	// Рекурсивное вычисление значения выражения
	result, evalErr := eval(node)
	if evalErr != nil {
		return "", evalErr
	}

	return strconv.FormatFloat(result, 'f', -1, 64), nil
}

func isValidExpression(expression string) bool {
	for _, r := range expression {
		if !(r >= '0' && r <= '9') && r != '+' && r != '-' && r != '*' && r != '/' && r != ' ' {
			return false
		}
	}
	return true
}

func eval(node interface{}) (float64, error) {
	// Реализация вычисления узла AST-дерева
	switch n := node.(type) {
	case *parser.BasicLit:
		return strconv.ParseFloat(n.Value, 64)
	case *parser.BinaryExpr:
		left, err := eval(n.X)
		if err != nil {
			return 0, err
		}
		right, err := eval(n.Y)
		if err != nil {
			return 0, err
		}
		switch n.Op.String() {
		case "+":
			return left + right, nil
		case "-":
			return left - right, nil
		case "*":
			return left * right, nil
		case "/":
			if right == 0 {
				return 0, errors.New("division by zero")
			}
			return left / right, nil
		}
	}
	return 0, errors.New("invalid expression")
}
