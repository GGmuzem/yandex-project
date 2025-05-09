package agent

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/GGmuzem/yandex-project/pkg/models"
)

// Agent представляет агента, который выполняет задачи
type Agent struct {
	ID         int
	grpcClient *GRPCClient
}

// Result представляет результат выполнения задачи
type Result struct {
	TaskID       int
	ExpressionID string
	Value        float64
	Success      bool
	ErrorMessage string
}

// NewAgent создает нового агента
func NewAgent(id int, serverAddr string) (*Agent, error) {
	client, err := NewGRPCClient(serverAddr, int32(id))
	if err != nil {
		return nil, err
	}

	return &Agent{
		ID:         id,
		grpcClient: client,
	}, nil
}

// Run запускает агента и начинает выполнение задач
func (a *Agent) Run(ctx context.Context) {
	log.Printf("Агент #%d: запущен (ID=%d)", a.grpcClient.agentID, a.ID)
	retryInterval := 2 * time.Second

	for {
		select {
		case <-ctx.Done():
			log.Printf("Агент #%d: получен сигнал остановки", a.grpcClient.agentID)
			return
		default:
			// Запрашиваем задачу от оркестратора
			task, err := a.grpcClient.GetTask()
			if err != nil {
				log.Printf("Агент #%d: ошибка при получении задачи: %v. Повторная попытка через %v",
					a.grpcClient.agentID, err, retryInterval)
				time.Sleep(retryInterval)
				continue
			}

			// Если задач нет, ждем и пробуем снова
			if (task == models.Task{}) {
				log.Printf("Агент #%d: нет задач, ожидание %v", a.grpcClient.agentID, retryInterval)
				time.Sleep(retryInterval)
				continue
			}

			// Вычисляем результат
			result := a.computeTask(task)

			// Имитируем длительное время вычисления, если указано
			if task.OperationTime > 0 {
				log.Printf("Агент #%d: имитация длительного вычисления задачи #%d (%d мс)",
					a.grpcClient.agentID, task.ID, task.OperationTime)
				time.Sleep(time.Duration(task.OperationTime) * time.Millisecond)
			}

			// Отправляем результат оркестратору
			taskResult := &models.TaskResult{
				ID:     result.TaskID,
				Result: result.Value,
			}
			err = a.grpcClient.SubmitResult(taskResult, task.ExpressionID)
			if err != nil {
				log.Printf("Агент #%d: ошибка при отправке результата задачи #%d: %v",
					a.grpcClient.agentID, task.ID, err)
				continue
			}
			log.Printf("Агент #%d: результат задачи #%d успешно отправлен (результат = %f)",
				a.grpcClient.agentID, task.ID, result.Value)
		}
	}
}

// computeTask вычисляет результат для данной задачи
func (a *Agent) computeTask(task models.Task) Result {
	log.Printf("Агент #%d: начало вычисления задачи #%d (операция: '%s', аргументы: '%s', '%s')",
		a.grpcClient.agentID, task.ID, task.Operation, task.Arg1, task.Arg2)

	var value float64
	var success bool
	var errorMsg string

	// Проверяем, что аргументы не пустые
	if task.Arg1 == "" || task.Arg2 == "" {
		errorMsg = "пустые аргументы в задаче"
		log.Printf("Агент #%d: %s #%d: '%s', '%s'", a.grpcClient.agentID, errorMsg, task.ID, task.Arg1, task.Arg2)
		return Result{
			TaskID:       task.ID,
			ExpressionID: task.ExpressionID,
			Value:        0,
			Success:      false,
			ErrorMessage: errorMsg,
		}
	}

	// Преобразуем аргументы в числа
	arg1, err1 := strconv.ParseFloat(task.Arg1, 64)
	arg2, err2 := strconv.ParseFloat(task.Arg2, 64)

	if err1 != nil || err2 != nil {
		errorMsg = fmt.Sprintf("ошибка преобразования аргументов: arg1=%v, arg2=%v", err1, err2)
		log.Printf("Агент #%d: %s", a.grpcClient.agentID, errorMsg)
		return Result{
			TaskID:       task.ID,
			ExpressionID: task.ExpressionID,
			Value:        0,
			Success:      false,
			ErrorMessage: errorMsg,
		}
	}

	// Выполняем операцию
	switch task.Operation {
	case "+":
		value = arg1 + arg2
		success = true
	case "-":
		value = arg1 - arg2
		success = true
	case "*":
		value = arg1 * arg2
		success = true
	case "/":
		if arg2 == 0 {
			errorMsg = "деление на ноль"
			success = false
		} else {
			value = arg1 / arg2
			success = true
		}
	default:
		errorMsg = fmt.Sprintf("неизвестная операция: %s", task.Operation)
		success = false
	}

	log.Printf("Агент #%d: завершено вычисление задачи #%d, результат: %f, успех: %v",
		a.grpcClient.agentID, task.ID, value, success)

	return Result{
		TaskID:       task.ID,
		ExpressionID: task.ExpressionID,
		Value:        value,
		Success:      success,
		ErrorMessage: errorMsg,
	}
}

// StartProcessing запускает обработку задач
func (a *Agent) StartProcessing() {
	for {
		// Получаем задачу от оркестратора
		task, err := a.grpcClient.GetTask()
		if err != nil {
			// Если ошибка - ждем и пробуем снова
			log.Printf("Воркер gRPC %d: Ошибка получения задачи: %v", a.grpcClient.agentID, err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Если произошла ошибка или нет задач, продолжаем опрашивать сервер
		if err != nil {
			log.Printf("Воркер gRPC %d: Ошибка получения задачи: %v", a.grpcClient.agentID, err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Проверяем, что задача не пустая
		if task.ID == 0 && task.Operation == "" && task.Arg1 == "" && task.Arg2 == "" {
			log.Printf("Воркер gRPC %d: Получена пустая задача, ожидание", a.grpcClient.agentID)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Обрабатываем задачу
		log.Printf("Воркер gRPC %d: Начало выполнения задачи #%d", a.grpcClient.agentID, task.ID)

		// Используем существующую функцию computeTask
		result := a.computeTask(task)

		// Если имитация длительных вычислений
		if task.OperationTime > 0 {
			log.Printf("Воркер gRPC %d: имитация длительного вычисления задачи #%d (%d мс)",
				a.grpcClient.agentID, task.ID, task.OperationTime)
			time.Sleep(time.Duration(task.OperationTime) * time.Millisecond)
		}

		// Формируем объект TaskResult в соответствии с определением из models.go
		taskResult := &models.TaskResult{
			ID:     result.TaskID,
			Result: result.Value,
		}

		log.Printf("Воркер gRPC %d: Успешно выполнена задача #%d с результатом %f",
			a.grpcClient.agentID, task.ID, result.Value)

		// Отправляем результат оркестратору, используя ExpressionID из результата
		if err := a.grpcClient.SubmitResult(taskResult, result.ExpressionID); err != nil {
			log.Printf("Воркер gRPC %d: Ошибка отправки результата задачи #%d: %v",
				a.grpcClient.agentID, task.ID, err)
		}
	}
}
