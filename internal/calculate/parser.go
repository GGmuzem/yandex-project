package calculate

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

// Evaluate принимает строковое выражение и возвращает результат
func Evaluate(expression string) (string, error) {
	// Проверяем наличие недопустимых символов
	if !isValidExpression(expression) {
		return "", errors.New("invalid expression")
	}

	// Разбираем выражение с использованием Go parser
	node, err := parser.ParseExpr(expression)
	if err != nil {
		return "", errors.New("expression is not valid")
	}

	// Рекурсивное вычисление значения выражения
	result, evalErr := eval(node)
	if evalErr != nil {
		return "", evalErr
	}

	return strconv.FormatFloat(result, 'f', -1, 64), nil
}

// eval рекурсивно вычисляет значение узла AST-дерева
func eval(node ast.Node) (float64, error) {
	switch n := node.(type) {
	case *ast.BasicLit: // Константа (число)
		return strconv.ParseFloat(n.Value, 64)

	case *ast.ParenExpr: // Обработка выражений в скобках
		return eval(n.X)

	case *ast.BinaryExpr: // Бинарное выражение
		left, err := eval(n.X)
		if err != nil {
			return 0, err
		}
		right, err := eval(n.Y)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case token.ADD:
			return left + right, nil
		case token.SUB:
			return left - right, nil
		case token.MUL:
			return left * right, nil
		case token.QUO:
			if right == 0 {
				return 0, errors.New("division by zero")
			}
			return left / right, nil
		default:
			return 0, errors.New("unsupported operator")
		}
	default:
		return 0, errors.New("unsupported expression")
	}
}

// isValidExpression проверяет строку на наличие недопустимых символов
func isValidExpression(expression string) bool {
	for _, r := range expression {
		if !(r >= '0' && r <= '9') && r != '+' && r != '-' && r != '*' && r != '/' && r != ' ' && r != '(' && r != ')' {
			return false
		}
	}
	return true
}
