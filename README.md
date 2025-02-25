# Распределённый вычислитель арифметических выражений

Проект для курса Яндекса. Система принимает арифметические выражения, разбивает их на задачи и вычисляет распределённо с помощью агентов.

## Архитектура системы

Система состоит из двух компонентов:

1. **Оркестратор** - сервер, который принимает арифметические выражения, разбирает их на отдельные операции и управляет процессом вычисления.
2. **Агент** - вычислитель, который получает задачи от оркестратора, выполняет их и возвращает результаты.

![Схема работы системы](https://mermaid.ink/img/pako:eNp1kU1vwjAMhv9KlNOQhnTYWHTiY4cfMKnACTiUxm2t5qOKU7GN8d-X0nYCiR0ivXn9xI4dXISSDKTQ7GyFpKrY6sIwJl6VRW20IyacZ0sI5tg7R9jZvD3Jqo1vC22qkiuFhGQ0deLg5FGRgz0GDNpLZxbEeBTewPAPiPSO_kTnb3gEIvPYGa2REO6EVQU5E80-Mc3CJDz-fA4xTFRN_G_Qd_yuxd42bnWjveTmO9Cd0DpndIHUDTLXdIFNpJWXtmsbuSqRx7vD9Tbd8QN4pOtzr6-kp1sH1cDnxiKbRDj2uKsyVjhdYWTDIdL6KPLBKNVujfTDvtMaFGSYKEH4OC_ZW3qnIPtm9CgluG3fNaQQV2Uo7Xu47voDTXiLjA?type=png)

## Требования

- Go 1.16 или выше
- Доступ к localhost для HTTP-запросов

## Установка и запуск

1. Клонировать репозиторий:
   ```bash
   git clone https://github.com/your-username/yandex-project.git
   cd yandex-project
   ```

2. **Запуск оркестратора**:
   ```bash
   go run ./cmd/orchestrator/main.go
   ```
   
   Оркестратор запустится на порту 8080 по умолчанию.

3. **Запуск агента** (в новом терминале):
   ```bash
   # Для Linux/Mac:
   COMPUTING_POWER=3 go run ./cmd/agent/main.go
   
   # Для Windows (PowerShell):
   $env:COMPUTING_POWER=3; go run ./cmd/agent/main.go
   
   # Для Windows (CMD):
   set COMPUTING_POWER=3
   go run ./cmd/agent/main.go
   ```

   Где `COMPUTING_POWER` - количество горутин (вычислительных потоков) агента.

4. **Настройка времени операций** (опционально):
   ```bash
   # Для Linux/Mac:
   TIME_ADDITION_MS=100 TIME_SUBTRACTION_MS=100 TIME_MULTIPLICATIONS_MS=200 TIME_DIVISIONS_MS=200 go run ./cmd/orchestrator/main.go
   
   # Для Windows (PowerShell):
   $env:TIME_ADDITION_MS=100; $env:TIME_SUBTRACTION_MS=100; $env:TIME_MULTIPLICATIONS_MS=200; $env:TIME_DIVISIONS_MS=200; go run ./cmd/orchestrator/main.go
   
   # Для Windows (CMD):
   set TIME_ADDITION_MS=100
   set TIME_SUBTRACTION_MS=100
   set TIME_MULTIPLICATIONS_MS=200
   set TIME_DIVISIONS_MS=200
   go run ./cmd/orchestrator/main.go
   ```

## Примеры использования API

1. **Добавить выражение**:
   ```bash
   # С помощью curl
   curl -X POST 'localhost:8080/api/v1/calculate' \
   -H 'Content-Type: application/json' \
   -d '{"expression": "2 + 2 * 2"}'
   
   # С помощью PowerShell
   Invoke-RestMethod -Method POST -Uri "http://localhost:8080/api/v1/calculate" -ContentType "application/json" -Body '{"expression": "2 + 2 * 2"}'
   ```

   Ответ: 
   ```json
   {"id": "expr0"}
   ```

2. **Получить список выражений**:
   ```bash
   # С помощью curl
   curl 'localhost:8080/api/v1/expressions'
   
   # С помощью PowerShell
   Invoke-RestMethod -Uri "http://localhost:8080/api/v1/expressions"
   ```

   Ответ (в процессе вычисления):
   ```json
   {"expressions": [{"id": "expr0", "status": "pending", "result": 0}]}
   ```

   Ответ (после вычисления):
   ```json
   {"expressions": [{"id": "expr0", "status": "completed", "result": 6}]}
   ```

3. **Получить конкретное выражение**:
   ```bash
   # С помощью curl
   curl 'localhost:8080/api/v1/expressions/expr0'
   
   # С помощью PowerShell
   Invoke-RestMethod -Uri "http://localhost:8080/api/v1/expressions/expr0"
   ```

   Ответ:
   ```json
   {"expression": {"id": "expr0", "status": "completed", "result": 6}}
   ```

4. **Ошибка: неверные данные**:
   ```bash
   # С помощью curl
   curl -X POST 'localhost:8080/api/v1/calculate' \
   -H 'Content-Type: application/json' \
   -d '{"expression": ""}'
   
   # С помощью PowerShell
   Invoke-RestMethod -Method POST -Uri "http://localhost:8080/api/v1/calculate" -ContentType "application/json" -Body '{"expression": ""}'
   ```

   Ответ: 422 Invalid data

## Особенности реализации

- Парсер выражений поддерживает основные арифметические операции (+, -, *, /) и скобки
- Приоритет операций соблюдается (умножение и деление имеют больший приоритет, чем сложение и вычитание)
- Поддерживается параллельное вычисление независимых частей выражения
- Время выполнения каждой операции настраивается через переменные окружения

## Лицензия

MIT