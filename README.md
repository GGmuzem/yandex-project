# Распределённый вычислитель арифметических выражений

Проект представляет собой распределённую систему для вычисления сложных арифметических выражений. Система эффективно разбивает выражения на атомарные операции, распределяет их между вычислительными агентами и собирает результаты, соблюдая все зависимости между операциями.

## Возможности системы

- Поддержка сложных математических выражений с множественными уровнями вложенности
- Автоматическое разбиение выражений на атомарные операции
- Параллельное выполнение независимых операций
- Корректная обработка зависимостей между операциями
- Масштабируемость через добавление дополнительных агентов
- Настраиваемое время выполнения операций
- REST API для взаимодействия с системой

## Архитектура системы

Система построена на основе архитектуры оркестратор-агент:

1. **Оркестратор**:
   - Принимает и парсит математические выражения
   - Разбивает выражения на атомарные операции
   - Управляет зависимостями между операциями
   - Распределяет задачи между агентами
   - Собирает и агрегирует результаты
   - Предоставляет REST API для внешнего взаимодействия

2. **Агент**:
   - Получает задачи от оркестратора
   - Выполняет атомарные математические операции
   - Возвращает результаты оркестратору
   - Поддерживает параллельное выполнение операций
   - Масштабируется через настройку количества воркеров



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

1. **Вычисление простого выражения**:
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
   {"id": "expr id:0"}
   ```

2. **Вычисление сложного выражения**:
   ```bash
   # С помощью PowerShell
   $body = @{
       "expression" = "((((25 / 5) * 3) + ((15 - 7) * 4)) / (((10 + 2) * 2) - 5)) * (((8 * 2) - 3) + ((20 / 4) * 3))"
   } | ConvertTo-Json
   
   Invoke-RestMethod -Method POST -Uri "http://localhost:8080/api/v1/calculate" -ContentType "application/json" -Body $body
   ```

3. **Получить список выражений**:
   ```bash
   # С помощью curl
   curl 'localhost:8080/api/v1/expressions'
   
   # С помощью PowerShell
   Invoke-RestMethod -Uri "http://localhost:8080/api/v1/expressions"
   ```

   Ответ (после вычисления):
   ```json
   {
     "expressions": [
       {"id": "expr id:0", "status": "completed", "result": 6},
       {"id": "expr id:1", "status": "completed", "result": 40}
     ]
   }
   ```

4. **Получить статус конкретного выражения**:
   ```bash
   # С помощью PowerShell
   Invoke-RestMethod -Uri "http://localhost:8080/api/v1/expressions/expr id:0"
   ```

   Ответ:
   ```json
   {"expression": {"id": "expr id:0", "status": "completed", "result": 6}}
   ```

## Особенности реализации

- **Парсер выражений**:
  - Поддержка сложных математических выражений
  - Корректная обработка вложенных скобок
  - Соблюдение приоритетов операций
  - Преобразование в постфиксную нотацию

- **Система управления задачами**:
  - Автоматическое определение зависимостей
  - Параллельное выполнение независимых операций
  - Эффективное распределение нагрузки
  - Отслеживание статуса выполнения

- **Масштабируемость**:
  - Возможность добавления новых агентов
  - Настройка количества воркеров
  - Конфигурируемое время операций
  - Балансировка нагрузки

