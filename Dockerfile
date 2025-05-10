FROM golang:1.23-alpine AS builder

# Устанавливаем необходимые зависимости
# sqlite3 требует gcc и musl-dev, а также sqlite-dev для компиляции
RUN apk add --no-cache git gcc musl-dev sqlite-dev

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем файлы go.mod и go.sum
COPY go.mod go.sum ./

# Скачиваем зависимости
RUN go mod download

# Копируем все исходные файлы
COPY . .

# Компилируем оркестратор с включенным CGO для поддержки SQLite
RUN CGO_ENABLED=1 GOOS=linux go build -a -o /go/bin/orchestrator cmd/orchestrator/main.go

# Компилируем агента
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /go/bin/agent cmd/agent/main.go

# Создаем финальный образ для оркестратора
FROM alpine:latest AS orchestrator
# Устанавливаем все необходимые зависимости для SQLite и работы CGO
RUN apk --no-cache add ca-certificates sqlite sqlite-libs sqlite-dev libc6-compat gcc musl-dev
# Создаем директорию для данных
WORKDIR /root/
RUN mkdir -p /root/data
COPY --from=builder /go/bin/orchestrator .
COPY --from=builder /app/web /root/web

# Указываем порт, который будет использоваться
EXPOSE 8081

# Задаем путь к базе данных в монтируемом томе
ENV DB_PATH=/root/data/calculator.db

# Запускаем оркестратор
CMD ["./orchestrator"]

# Создаем финальный образ для агента
FROM alpine:latest AS agent
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /go/bin/agent .

# Задаем вычислительную мощность по умолчанию
ENV COMPUTING_POWER=4

# Запускаем агента
CMD ["./agent"]
