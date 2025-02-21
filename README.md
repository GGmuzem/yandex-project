
# Распределённый вычислитель арифметических выражений

Проект для курса Яндекса. Система принимает арифметические выражения, разбивает их на задачи и вычисляет распределённо с помощью агентов.

## Запуск

1. **Оркестратор**:
   ```bash
   go run ./cmd/orchestrator/main.go

2. **Агент** (запуск с 3 горутинами):
    ```bash
    set COMPUTING_POWER=3 
    go run ./cmd/agent/main.go

## Примеры использования

1. **Добавить выражение:**
    ```bash
    curl -X POST 'localhost:8080/api/v1/calculate' \
    -H 'Content-Type: application/json' \
    -d '{"expression": "2 + 2 * 2"}'

    Ответ: {"id": "expr0"}

2. **Получить список выражений:**
    ```bash
    curl 'localhost:8080/api/v1/expressions'

    Ответ:
    {"expressions": [{"id": "expr0", "status": "pending", "result": 0}]}

3. **Получить конкретное выражение:**
    ```bash
    curl 'localhost:8080/api/v1/expressions/expr0'

    Ответ:
    {"expression": {"id": "expr0", "status": "completed", "result": 6}}

4. **Ошибка: неверные данные:**
    ```bash
    curl -X POST 'localhost:8080/api/v1/calculate' \
    -H 'Content-Type: application/json' \
    -d '{"expression": ""}'

    Ответ: 422 Invalid data