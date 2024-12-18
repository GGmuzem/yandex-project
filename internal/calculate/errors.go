package calculate

import "fmt"

// CalcError описывает пользовательскую ошибку обработки выражения
type CalcError struct {
	Message string
}

func (e *CalcError) Error() string {
	return e.Message
}

// NewCalcError создает новую ошибку CalcError
func NewCalcError(message string) *CalcError {
	return &CalcError{Message: message}
}

// DivisionByZeroError создаёт ошибку деления на ноль
func DivisionByZeroError() *CalcError {
	return NewCalcError("Division by zero")
}

// InvalidExpressionError создаёт ошибку некорректного выражения
func InvalidExpressionError(expression string) *CalcError {
	return NewCalcError(fmt.Sprintf("Invalid expression: %s", expression))
}

