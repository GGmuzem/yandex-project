package orchestrator

import (
	"context"
	"log"
	"net"
	"os"
	"sync"

	"github.com/GGmuzem/yandex-project/internal/database"
	"github.com/GGmuzem/yandex-project/pkg/calculator"
	"github.com/GGmuzem/yandex-project/pkg/models"
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
	log.Printf("=== GRPC SERVER: Получен запрос GetTask от агента ID=%d", req.AgentID)

	// Используем метод GetTask из менеджера задач
	task, found := Manager.GetTask()

	// Если нет готовых задач, возвращаем пустую задачу
	if !found {
		log.Printf("=== GRPC SERVER: Нет готовых задач для агента #%d", req.AgentID)
		return &calculator.Task{
			ID:            0,
			Arg1:          "",
			Arg2:          "",
			Operation:     "",
			OperationTime: 0,
			ExpressionID:  "",
		}, nil
	}

	log.Printf("=== GRPC SERVER: Возвращаем задачу #%d агенту #%d: %s %s %s", 
		task.ID, req.AgentID, task.Arg1, task.Operation, task.Arg2)

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
	log.Printf("=== GRPC SERVER: Получен результат для задачи #%d: %f, выражение: %s", result.ID, result.Result, result.ExpressionID)

	// Создаем объект TaskResult для передачи в менеджер задач
	taskResult := models.TaskResult{
		ID:     int(result.ID),
		Result: result.Result,
	}

	// Добавляем результат в менеджер задач
	success := Manager.AddResult(taskResult)
	if !success {
		log.Printf("=== GRPC SERVER: Ошибка при добавлении результата задачи #%d", result.ID)
		return &calculator.SubmitResultResponse{
			Success: false,
			Message: "ошибка при обработке результата",
		}, nil
	}

	// Получаем ID выражения
	exprID := result.ExpressionID
	if exprID == "" {
		log.Printf("=== GRPC SERVER: Задача #%d не имеет связанного выражения", result.ID)
		return &calculator.SubmitResultResponse{
			Success: true,
			Message: "результат обработан",
		}, nil
	}

	// Сохраняем результат в БД
	if err := s.DB.SaveResult(int(result.ID), result.Result, exprID); err != nil {
		log.Printf("=== GRPC SERVER: Ошибка при сохранении результата задачи #%d в БД: %v", result.ID, err)
	} else {
		log.Printf("=== GRPC SERVER: Результат задачи #%d успешно сохранен в БД", result.ID)
	}

	// Обновляем статусы выражений
	UpdateExpressions()

	log.Printf("=== GRPC SERVER: Результат задачи #%d успешно обработан", result.ID)
	
	return &calculator.SubmitResultResponse{
		Success: true,
		Message: "результат обработан",
	}, nil
}
