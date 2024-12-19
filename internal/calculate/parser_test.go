package calculate

import "testing"

func TestEvaluate(t *testing.T) {
	tests := []struct {
		expression string
		expected   string
		shouldFail bool
	}{
		{"2+2", "4", false},
		{"2+2*2", "6", false},
		{"10/2", "5", false},
		{"10/0", "", true},        // Деление на ноль
		{"2++2", "", true},        // Некорректное выражение
		{"(2+3)*4", "20", false},  // Скобки
		{"abc", "", true},         // Некорректные символы
		{"2 + 3 / 1", "5", false}, // Пробелы
	}

	for _, test := range tests {
		result, err := Evaluate(test.expression)
		if test.shouldFail {
			if err == nil {
				t.Errorf("Expression '%s' expected to fail but got result '%s'", test.expression, result)
			}
		} else {
			if err != nil {
				t.Errorf("Expression '%s' failed unexpectedly: %v", test.expression, err)
			}
			if result != test.expected {
				t.Errorf("Expression '%s': expected '%s', got '%s'", test.expression, test.expected, result)
			}
		}
	}
}
