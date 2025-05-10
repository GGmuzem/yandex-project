package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/GGmuzem/yandex-project/pkg/calculator"
	"github.com/GGmuzem/yandex-project/pkg/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCClient клиент для gRPC взаимодействия с оркестратором
type GRPCClient struct {
	client  calculator.CalculatorClient
	conn    *grpc.ClientConn
	agentID int32
}

// NewGRPCClient создает новый gRPC клиент
func NewGRPCClient(serverAddr string, agentID int32) (*GRPCClient, error) {
	// Создаем соединение без TLS
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	// Создаем клиента
	client := calculator.NewCalculatorClient(conn)

	return &GRPCClient{
		client:  client,
		conn:    conn,
		agentID: agentID,
	}, nil
}

// Close закрывает соединение с сервером
func (c *GRPCClient) Close() error {
	return c.conn.Close()
}

// GetTask получает задачу от оркестратора
func (gc *GRPCClient) GetTask() (models.Task, error) {
	log.Printf("Агент #%d: Запрос задачи от сервера", gc.agentID)
	
	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Запрашиваем задачу с повторными попытками в случае ошибки
	var resp *calculator.Task
	var err error
	
	// Максимальное количество попыток
	maxRetries := 3
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Проверяем, что клиент инициализирован
		if gc.client == nil {
			log.Printf("Агент #%d: Ошибка - GRPC-клиент не инициализирован", gc.agentID)
			return models.Task{}, fmt.Errorf("GRPC-клиент не инициализирован")
		}
		
		log.Printf("Агент #%d: Отправка GetTask запроса (попытка %d)", gc.agentID, attempt)
		resp, err = gc.client.GetTask(ctx, &calculator.GetTaskRequest{
			AgentID: int32(gc.agentID),
		})
		
		if err == nil {
			log.Printf("Агент #%d: Успешно получен ответ от сервера", gc.agentID)
			break // Успешно получили ответ
		}
		
		log.Printf("Агент #%d: Ошибка при получении задачи: %v (попытка %d из %d)", 
			gc.agentID, err, attempt, maxRetries)
		
		if attempt < maxRetries {
			// Ждем перед следующей попыткой
			backoff := time.Duration(attempt*500) * time.Millisecond
			log.Printf("Агент #%d: Ожидание %v перед повторной попыткой", gc.agentID, backoff)
			time.Sleep(backoff)
		}
	}
	
	if err != nil {
		return models.Task{}, fmt.Errorf("не удалось получить задачу: %w", err)
	}
	
	log.Printf("Агент #%d: Получен ответ от сервера: ID=%d, Arg1='%s', Arg2='%s', Operation='%s', ExpressionID='%s', OperationTime=%d", 
		gc.agentID, resp.ID, resp.Arg1, resp.Arg2, resp.Operation, resp.ExpressionID, resp.OperationTime)
	
	// Проверяем, что задача не пустая
	if resp.ID == 0 && resp.Operation == "" {
		log.Printf("Агент #%d: Получен пустой ответ от сервера (нет готовых задач), запрошу задачу позже", gc.agentID)
		return models.Task{}, nil
	}
	
	// Преобразуем в нашу модель задачи
	task := models.Task{
		ID:            int(resp.ID),
		Arg1:          resp.Arg1,
		Arg2:          resp.Arg2,
		Operation:     resp.Operation,
		ExpressionID:  resp.ExpressionID,
		OperationTime: int(resp.OperationTime),
	}

	log.Printf("Агент #%d: Успешно получена задача: ID=%d, Arg1='%s', Arg2='%s', Operation='%s', ExpressionID='%s'",
		gc.agentID, task.ID, task.Arg1, task.Arg2, task.Operation, task.ExpressionID)

	return task, nil
}

// SubmitResult отправляет результат задачи оркестратору
func (c *GRPCClient) SubmitResult(result *models.TaskResult, expressionID string) error {
	// Увеличиваем таймаут для большей надежности
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Преобразуем результат в protobuf формат
	pbResult := &calculator.TaskResult{
		ID:           int32(result.ID),
		Result:       result.Result,
		ExpressionID: expressionID,
	}

	log.Printf("Агент #%d: Отправка результата задачи #%d: %f, выражение: %s", 
		c.agentID, result.ID, result.Result, expressionID)

	// Добавляем механизм повторных попыток при ошибке связи
	var resp *calculator.SubmitResultResponse
	var err error
	maxRetries := 5
	for retries := 0; retries < maxRetries; retries++ {
		// Отправляем результат
		resp, err = c.client.SubmitResult(ctx, pbResult)
		if err == nil {
			break
		}
		log.Printf("Агент #%d: Ошибка отправки результата задачи #%d (попытка %d/%d): %v", 
			c.agentID, result.ID, retries+1, maxRetries, err)
		if retries < maxRetries-1 {
			// Увеличиваем интервал между попытками (экспоненциальный бэкофф)
			backoff := time.Duration(500*(1<<retries)) * time.Millisecond
			log.Printf("Агент #%d: Ожидание %v перед следующей попыткой", c.agentID, backoff)
			time.Sleep(backoff)
		}
	}

	if err != nil {
		log.Printf("Агент #%d: Не удалось отправить результат задачи #%d после %d попыток: %v", 
			c.agentID, result.ID, maxRetries, err)
		return err
	}

	log.Printf("Агент #%d: Ответ сервера на результат задачи #%d: success=%v, message=%s", 
		c.agentID, result.ID, resp.Success, resp.Message)

	if !resp.Success {
		log.Printf("Агент #%d: Сервер отклонил результат задачи #%d: %s", 
			c.agentID, result.ID, resp.Message)
		return fmt.Errorf("сервер отклонил результат: %s", resp.Message)
	}

	log.Printf("Агент #%d: Результат задачи #%d успешно отправлен", c.agentID, result.ID)
	return nil
}

// StartGRPCWorker запускает воркер, взаимодействующий с оркестратором через gRPC
func StartGRPCWorker(id int, serverAddr string) {
	// Создаем клиента gRPC
	client, err := NewGRPCClient(serverAddr, int32(id))
	if err != nil {
		log.Fatalf("Ошибка создания gRPC клиента: %v", err)
	}
	defer client.Close()

	log.Printf("Воркер gRPC %d: запущен, подключен к серверу %s", id, serverAddr)

	// Интервал между запросами при отсутствии задач
	retryInterval := 1000 * time.Millisecond

	// Отладочный код: пробуем напрямую запросить задачу один раз
	task, err := client.GetTask()
	if err == nil && task.ID > 0 {
		log.Printf("Воркер gRPC %d: ТЕСТ - успешно получена задача #%d: %s %s %s",
			id, task.ID, task.Arg1, task.Operation, task.Arg2)

		// Вычисляем результат
		result := computeTask(task)

		// Результат задачи
		taskResult := &models.TaskResult{
			ID:     task.ID,
			Result: result,
		}

		// Отправляем результат
		err = client.SubmitResult(taskResult, task.ExpressionID)
		if err != nil {
			log.Printf("Воркер gRPC %d: ТЕСТ - ошибка отправки результата: %v", id, err)
		} else {
			log.Printf("Воркер gRPC %d: ТЕСТ - результат успешно отправлен: %f", id, result)
		}
	} else {
		log.Printf("Воркер gRPC %d: ТЕСТ - не удалось получить задачу: %v", id, err)
	}

	for {
		// Запрашиваем задачу от оркестратора
		task, err := client.GetTask()
		if err != nil {
			log.Printf("Воркер gRPC %d: ошибка получения задачи: %v", id, err)
			// Увеличиваем интервал при ошибке связи
			time.Sleep(retryInterval * 2)
			continue
		}

		// Если задач нет, ждем и пробуем снова
		if task == (models.Task{}) {
			// Динамически регулируем интервал опроса в зависимости от загрузки
			log.Printf("Воркер gRPC %d: нет готовых задач, ожидание %v", id, retryInterval)
			time.Sleep(retryInterval)
			// Постепенно увеличиваем интервал при отсутствии задач, но не более 5 секунд
			if retryInterval < 5*time.Second {
				retryInterval += 100 * time.Millisecond
			}
			continue
		} else {
			// Сбрасываем интервал при получении задачи
			retryInterval = 1000 * time.Millisecond
		}

		// Проверка на пустые аргументы
		if task.Arg1 == "" || task.Arg2 == "" {
			log.Printf("Воркер gRPC %d: ошибка - пустые аргументы в задаче #%d: '%s', '%s'", id, task.ID, task.Arg1, task.Arg2)
			time.Sleep(retryInterval)
			continue
		}

		log.Printf("Воркер gRPC %d: получена задача #%d: %s %s %s, ExprID=%s", 
			id, task.ID, task.Arg1, task.Operation, task.Arg2, task.ExpressionID)

		// Вычисляем результат
		result := computeTask(task)

		// Имитируем длительное время вычисления
		if task.OperationTime > 0 {
			log.Printf("Воркер gRPC %d: выполняется задача #%d (%d мс)...", id, task.ID, task.OperationTime)
			time.Sleep(time.Duration(task.OperationTime) * time.Millisecond)
		}

		// Результат задачи
		taskResult := &models.TaskResult{
			ID:     task.ID,
			Result: result,
		}

		log.Printf("Воркер gRPC %d: готов результат задачи #%d: %f", id, task.ID, result)

		// Отправляем результат через улучшенный метод SubmitResult
		err = client.SubmitResult(taskResult, task.ExpressionID)
		if err != nil {
			log.Printf("Воркер gRPC %d: не удалось отправить результат задачи #%d: %v", id, task.ID, err)
			// При ошибке отправки результата делаем паузу перед следующей задачей
			time.Sleep(retryInterval)
		} else {
			log.Printf("Воркер gRPC %d: задача #%d успешно завершена, результат: %f", id, task.ID, result)
		}
	}
}
