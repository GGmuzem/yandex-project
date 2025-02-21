package tests

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/GGmuzem/yandex-project/internal/orchestrator"
)

func TestCalculateHandler(t *testing.T) {
    // Тест успешного добавления выражения
    reqBody := `{"expression": "2 + 2 * 2"}`
    req, _ := http.NewRequest("POST", "/api/v1/calculate", bytes.NewBuffer([]byte(reqBody)))
    req.Header.Set("Content-Type", "application/json")
    rr := httptest.NewRecorder()
    handler := http.HandlerFunc(orchestrator.CalculateHandler)
    handler.ServeHTTP(rr, req)

    if status := rr.Code; status != http.StatusCreated {
        t.Errorf("Expected status %v, got %v", http.StatusCreated, status)
    }

    var resp map[string]string
    json.Unmarshal(rr.Body.Bytes(), &resp)
    if _, ok := resp["id"]; !ok {
        t.Errorf("Expected response with 'id', got %v", rr.Body.String())
    }

    // Тест неверных данных
    reqBody = `{"expression": ""}`
    req, _ = http.NewRequest("POST", "/api/v1/calculate", bytes.NewBuffer([]byte(reqBody)))
    req.Header.Set("Content-Type", "application/json")
    rr = httptest.NewRecorder()
    handler.ServeHTTP(rr, req)

    if status := rr.Code; status != http.StatusUnprocessableEntity {
        t.Errorf("Expected status %v, got %v", http.StatusUnprocessableEntity, status)
    }
}

func TestListExpressionsHandler(t *testing.T) {
    req, _ := http.NewRequest("GET", "/api/v1/expressions", nil)
    rr := httptest.NewRecorder()
    handler := http.HandlerFunc(orchestrator.ListExpressionsHandler)
    handler.ServeHTTP(rr, req)

    if status := rr.Code; status != http.StatusOK {
        t.Errorf("Expected status %v, got %v", http.StatusOK, status)
    }

    var resp map[string][]interface{}
    json.Unmarshal(rr.Body.Bytes(), &resp)
    if _, ok := resp["expressions"]; !ok {
        t.Errorf("Expected 'expressions' field, got %v", rr.Body.String())
    }
}

