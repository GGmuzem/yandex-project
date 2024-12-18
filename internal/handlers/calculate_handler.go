package handlers

import (
	"calc_service/internal/calculate"
	"encoding/json"
	"errors"
	"net/http"
)

// RequestBody структура для входных данных
type RequestBody struct {
	Expression string `json:"expression"`
}

// Response структура для успешного ответа
type Response struct {
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// CalculateHandler обрабатывает POST-запросы с арифметическими выражениями
func CalculateHandler(w http.ResponseWriter, r *http.Request) {
	// Проверка метода
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Парсинг тела запроса
	var reqBody RequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{Error: "Invalid JSON"})
		return
	}

	// Проверка валидности выражения и вычисление
	result, err := calculate.Evaluate(reqBody.Expression)
	if err != nil {
		handleError(w, err)
		return
	}

	// Успешный ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{Result: result})
}

func handleError(w http.ResponseWriter, err error) {
	var calcErr *calculate.CalcError
	if errors.As(err, &calcErr) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(Response{Error: "Expression is not valid"})
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Error: "Internal server error"})
	}
}
