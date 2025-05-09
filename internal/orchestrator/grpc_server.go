package orchestrator

import (
	"context"
	"log"
	"net"
	"os"
	"sync"

	"github.com/GGmuzem/yandex-project/internal/database"
	"github.com/GGmuzem/yandex-project/pkg/calculator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// CalculatorServer реализация gRPC сервера оркестратора
type CalculatorServer struct {
	DB         database.Database
	mutex      *sync.Mutex
	tasksMutex *sync.Mutex
}

// NewCalculatorServer создает новый gRPC сервер оркестратора
func NewCalculatorServer(db database.Database, mutex, tasksMutex *sync.Mutex) *CalculatorServer {
	return &CalculatorServer{
		DB:         db,
		mutex:      mutex,
		tasksMutex: tasksMutex,
	}
}

// StartGRPCServer запускает gRPC сервер оркестратора
func StartGRPCServer(db database.Database, mutex, tasksMutex *sync.Mutex) error {
	// Получаем порт из переменной окружения
	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50052" // По умолчанию используем порт 50052
	}

	// Создаем листенер для gRPC сервера
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}

	// Создаем gRPC сервер
	grpcServer := grpc.NewServer()

	// Регистрируем сервис оркестратора
	calculatorServer := NewCalculatorServer(db, mutex, tasksMutex)

	// Регистрируем сервер (регистрация должна быть определена в pkg/calculator)
	calculator.RegisterCalculatorServer(grpcServer, calculatorServer)

	// Включаем рефлексию для отладки
	reflection.Register(grpcServer)

	// Запускаем сервер в отдельной горутине
	go func() {
		log.Printf("gRPC сервер запущен на порту :%s", port)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Ошибка при запуске gRPC сервера: %v", err)
		}
	}()

	return nil
}

// GetTask - получение задачи агентом
func (s *CalculatorServer) GetTask(ctx context.Context, req *calculator.GetTaskRequest) (*calculator.Task, error) {
	log.Printf("=== ОТЛАДКА SERVER: Получен запрос GetTask от агента ID=%d", req.AgentID)

	// Добавляем дополнительное логирование для отладки
	log.Printf("=== ОТЛАДКА SERVER: Перед блокировкой - длина очереди готовых задач: %d", len(Manager.ReadyTasks))

	// Блокируем доступ к менеджеру задач
	Manager.mu.Lock()
	defer Manager.mu.Unlock()

	// Обновляем очередь готовых задач
	log.Printf("=== ОТЛАДКА SERVER: Проверка и обновление готовых задач")
	for taskID, taskPtr := range Manager.Tasks {
		if !Manager.ProcessingTasks[taskID] {
			task := *taskPtr
			if IsTaskReady(task) {
				// Проверяем, что задача ещё не в очереди
				var found bool
				for _, rt := range Manager.ReadyTasks {
					if rt.ID == task.ID {
						found = true
						break
					}
				}
				if !found {
					log.Printf("=== ОТЛАДКА SERVER: Добавляем задачу #%d в очередь", task.ID)
					Manager.ReadyTasks = append(Manager.ReadyTasks, task)
				}
			}
		}
	}

	log.Printf("=== ОТЛАДКА SERVER: Длина очереди готовых задач: %d", len(Manager.ReadyTasks))
	
	// Детальный вывод всех задач в очереди
	for i, task := range Manager.ReadyTasks {
		log.Printf("=== ОТЛАДКА SERVER: Готовая задача #%d в очереди (индекс %d): ID=%d, %s %s %s, ExprID=%s", 
			task.ID, i, task.ID, task.Arg1, task.Operation, task.Arg2, task.ExpressionID)
	}
	
	// Детальный вывод всех задач в системе
	log.Printf("=== ОТЛАДКА SERVER: Всего задач в системе: %d", len(Manager.Tasks))
	for taskID, taskPtr := range Manager.Tasks {
		if taskPtr != nil {
			task := *taskPtr
			log.Printf("=== ОТЛАДКА SERVER: Задача #%d: %s %s %s, ExprID=%s, В обработке=%v", 
				taskID, task.Arg1, task.Operation, task.Arg2, task.ExpressionID, Manager.ProcessingTasks[taskID])
		}
	}

	// Если нет готовых задач, возвращаем пустую задачу
	if len(Manager.ReadyTasks) == 0 {
		log.Printf("=== ОТЛАДКА SERVER: Нет готовых задач для агента #%d", req.AgentID)
		return &calculator.Task{
			ID:            0,
			Arg1:          "",
			Arg2:          "",
			Operation:     "",
			OperationTime: 0,
			ExpressionID:  "",
		}, nil
	}

	// Выбираем первую задачу из очереди
	task := Manager.ReadyTasks[0]
	// Удаляем её из очереди
	Manager.ReadyTasks = Manager.ReadyTasks[1:]
	// Отмечаем задачу как обрабатываемую
	Manager.ProcessingTasks[task.ID] = true

	log.Printf("=== ОТЛАДКА SERVER: Возвращаем задачу #%d агенту #%d", task.ID, req.AgentID)

	// Преобразуем в protobuf формат
	return &calculator.Task{
		ID:            int32(task.ID),
		Arg1:          task.Arg1,
		Arg2:          task.Arg2,
		Operation:     task.Operation,
		OperationTime: int32(task.OperationTime),
		ExpressionID:  task.ExpressionID,
	}, nil
}

// SubmitResult обрабатывает результат задачи от агента
func (s *CalculatorServer) SubmitResult(ctx context.Context, result *calculator.TaskResult) (*calculator.SubmitResultResponse, error) {
	log.Printf("Получен результат для задачи #%d: %f, выражение: %s", result.ID, result.Result, result.ExpressionID)

	// Сохраняем результат в память
	Manager.mu.Lock()
	Manager.Results[int(result.ID)] = result.Result
	log.Printf("Результат задачи #%d сохранен в памяти: %f", result.ID, result.Result)
	Manager.mu.Unlock()

	// Удаляем задачу из списка задач в обработке
	Manager.mu.Lock()
	delete(Manager.ProcessingTasks, int(result.ID))
	delete(Manager.Tasks, int(result.ID))
	Manager.mu.Unlock()

	// Получаем ID выражения
	exprID := result.ExpressionID
	if exprID == "" {
		log.Printf("Задача #%d не имеет связанного выражения", result.ID)
		return &calculator.SubmitResultResponse{
			Success: true,
			Message: "результат обработан",
		}, nil
	}

	// Сохраняем результат в БД
	if err := s.DB.SaveResult(int(result.ID), result.Result, exprID); err != nil {
		log.Printf("Ошибка при сохранении результата задачи #%d в БД: %v", result.ID, err)
	} else {
		log.Printf("Результат задачи #%d успешно сохранен в БД", result.ID)
	}

	// Обновляем готовые задачи, которые зависят от этого результата
	updateReadyTasks()

	// Обновляем статусы выражений
	UpdateExpressions()

	return &calculator.SubmitResultResponse{
		Success: true,
		Message: "результат обработан",
	}, nil
}
