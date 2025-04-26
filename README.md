# Калькулятор выражений

Проект представляет собой распределенную систему для вычисления математических выражений, состоящую из оркестратора и агентов.

## Компоненты системы

1. **Оркестратор** - центральный компонент, который:
   - Разбирает математические выражения на отдельные задачи
   - Управляет очередью задач
   - Распределяет задачи между агентами
   - Предоставляет API для взаимодействия с пользователями

2. **Агенты** - вычислительные узлы, которые:
   - Подключаются к оркестратору по gRPC
   - Выполняют задачи, полученные от оркестратора
   - Возвращают результаты вычислений обратно в оркестратор

3. **Простой сервер** - упрощенная версия оркестратора для тестирования API

## Возможности

- Регистрация и авторизация пользователей с JWT-токенами
- Вычисление математических выражений
- Сохранение истории выражений и их результатов
- Web-интерфейс для удобного использования
- Масштабирование системы за счет добавления агентов
- Поддержка параллельных вычислений

## Сборка и запуск

### Сборка исполняемых файлов

```bash
go build -o orchestrator.exe ./cmd/orchestrator
go build -o agent.exe ./cmd/agent
go build -o simple_server.exe simple_server.go
```

### Запуск без Docker

1. Запуск оркестратора:
```bash
./orchestrator.exe
```

2. Запуск агента:
```bash
./agent.exe
```

3. Запуск упрощенного сервера:
```bash
./simple_server.exe
```

### Запуск с Docker

1. Сборка и запуск контейнеров:
```bash
docker-compose up -d
```

2. Остановка контейнеров:
```bash
docker-compose down
```

## API

### Регистрация

```
POST /api/v1/register
Content-Type: application/json

{
  "login": "username",
  "password": "password"
}
```

### Авторизация

```
POST /api/v1/login
Content-Type: application/json

{
  "login": "username",
  "password": "password"
}
```

Ответ содержит JWT-токен:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### Вычисление выражения

```
POST /api/v1/calculate
Content-Type: application/json
Authorization: Bearer <token>

{
  "expression": "2+2*2"
}
```

Ответ содержит ID выражения:
```json
{
  "id": "expr-123"
}
```

### Получение результата выражения

```
GET /api/v1/expressions/expr-123
Authorization: Bearer <token>
```

Ответ содержит информацию о выражении:
```json
{
  "id": "expr-123",
  "status": "completed",
  "result": 6
}
```

### Получение списка выражений

```
GET /api/v1/expressions
Authorization: Bearer <token>
```

## Примеры использования (PowerShell)

```powershell
# Регистрация
$registerResponse = Invoke-WebRequest -Method POST -Uri "http://localhost:8080/api/v1/register" -ContentType "application/json" -Body '{"login":"test","password":"password"}'

# Авторизация
$loginResponse = Invoke-WebRequest -Method POST -Uri "http://localhost:8080/api/v1/login" -ContentType "application/json" -Body '{"login":"test","password":"password"}'
$token = ($loginResponse.Content | ConvertFrom-Json).token

# Расчет выражения
$calcResponse = Invoke-WebRequest -Method POST -Uri "http://localhost:8080/api/v1/calculate" -ContentType "application/json" -Headers @{"Authorization"="Bearer $token"} -Body '{"expression":"2+2"}'
$exprId = ($calcResponse.Content | ConvertFrom-Json).id

# Получение результата
Invoke-WebRequest -Method GET -Uri "http://localhost:8080/api/v1/expressions/$exprId" -Headers @{"Authorization"="Bearer $token"}

# Получение списка выражений
Invoke-WebRequest -Method GET -Uri "http://localhost:8080/api/v1/expressions" -Headers @{"Authorization"="Bearer $token"}
```

## Особенности реализации

1. **Многопользовательский режим** - каждый пользователь имеет свою историю выражений
2. **Персистентность данных** - все данные сохраняются в БД SQLite
3. **gRPC коммуникация** - взаимодействие между оркестратором и агентами
4. **In-memory режим** - возможность работы без SQLite (без CGO)
5. **Docker-контейнеры** - готовая к развертыванию система

## Требования

- Go 1.21 или выше
- Для полной функциональности с SQLite необходим CGO (компилятор C)
- Для запуска в Docker необходим Docker и Docker Compose
