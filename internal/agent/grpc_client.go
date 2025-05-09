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
	req := &calculator.GetTaskRequest{
		AgentID: gc.agentID,
	}
	log.Printf("Агент #%d: Запрос задачи от сервера", gc.agentID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := gc.client.GetTask(ctx, req)
	if err != nil {
		log.Printf("Агент #%d: Ошибка при получении задачи: %v", gc.agentID, err)
		return models.Task{}, err
	}

	log.Printf("Агент #%d: Получен ответ от сервера: ID=%d, Arg1='%s', Arg2='%s', Operation='%s', ExpressionID='%s', OperationTime=%d",
		gc.agentID, resp.ID, resp.Arg1, resp.Arg2, resp.Operation, resp.ExpressionID, resp.OperationTime)

	// Улучшенная проверка на пустой ответ от сервера - проверка всех полей
	if resp.ID == 0 && resp.Arg1 == "" && resp.Arg2 == "" && resp.Operation == "" {
		log.Printf("Агент #%d: Получен пустой ответ от сервера (нет задач), запрошу задачу позже", gc.agentID)
		return models.Task{}, nil
	}

	// Дополнительная проверка на валидность задачи
	if resp.Arg1 == "" || resp.Arg2 == "" || resp.Operation == "" {
		log.Printf("Агент #%d: Получена невалидная задача: пустые аргументы или операция", gc.agentID)
		return models.Task{}, fmt.Errorf("невалидная задача: пустые аргументы или операция")
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

	log.Printf("Агент #%d: Преобразовал в модель задачи: ID=%d, Arg1='%s', Arg2='%s', Operation='%s', ExpressionID='%s'",
		gc.agentID, task.ID, task.Arg1, task.Arg2, task.Operation, task.ExpressionID)

	return task, nil
}

// SubmitResult отправляет результат задачи оркестратору
func (c *GRPCClient) SubmitResult(result *models.TaskResult, expressionID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Преобразуем результат в protobuf формат
	pbResult := &calculator.TaskResult{
		ID:           int32(result.ID),
		Result:       result.Result,
		ExpressionID: expressionID,
	}

	// Отправляем результат
	resp, err := c.client.SubmitResult(ctx, pbResult)
	if err != nil {
		log.Printf("Ошибка отправки результата: %v", err)
		return err
	}
	log.Printf("SubmitResultResponse от сервера: success=%v, message=%s", resp.Success, resp.Message)
	if !resp.Success {
		return fmt.Errorf("SubmitResult failed: %s", resp.Message)
	}

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
	// Максимальное количество попыток отправки результата
	maxRetries := 5

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
			time.Sleep(retryInterval)
			continue
		}

		// Если задач нет, ждем и пробуем снова
		if task == (models.Task{}) {
			time.Sleep(retryInterval)
			continue
		}

		// Проверка на пустые аргументы
		if task.Arg1 == "" || task.Arg2 == "" {
			log.Printf("Ошибка: пустые аргументы в задаче #%d: '%s', '%s'", task.ID, task.Arg1, task.Arg2)
			time.Sleep(retryInterval)
			continue
		}

		log.Printf("Воркер gRPC %d: получена задача #%d: %s %s %s", id, task.ID, task.Arg1, task.Operation, task.Arg2)

		// Вычисляем результат
		result := computeTask(task)

		// Имитируем длительное время вычисления
		log.Printf("Воркер gRPC %d: выполняется задача #%d (%d мс)...", id, task.ID, task.OperationTime)
		time.Sleep(time.Duration(task.OperationTime) * time.Millisecond)

		// Результат задачи
		taskResult := &models.TaskResult{
			ID:     task.ID,
			Result: result,
		}

		log.Printf("Воркер gRPC %d: готов результат задачи #%d: %f", id, task.ID, result)

		// Механизм повторных попыток отправки результата
		retryCount := 0
		success := false

		for retryCount < maxRetries && !success {
			if retryCount > 0 {
				log.Printf("Воркер gRPC %d: повторная попытка #%d отправки результата задачи #%d", id, retryCount, task.ID)
				time.Sleep(retryInterval * time.Duration(retryCount)) // Увеличиваем интервал с каждой попыткой
			}

			err := client.SubmitResult(taskResult, task.ExpressionID)
			if err != nil {
				log.Printf("Воркер gRPC %d: ошибка отправки результата (попытка %d): %v", id, retryCount+1, err)
				retryCount++
				continue
			}

			success = true
			log.Printf("Воркер gRPC %d: задача #%d успешно завершена, результат: %f", id, task.ID, result)
			break
		}

		if !success {
			log.Printf("Воркер gRPC %d: не удалось отправить результат задачи #%d после %d попыток",
				id, task.ID, maxRetries)
		}
	}
}
