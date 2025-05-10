package orchestrator

import (
	"context"
	"log"
	"net"
	"os"
	"sync"
	"time"

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

	// Добавляем прямой доступ к менеджеру задач
	Manager.mu.Lock()
	defer Manager.mu.Unlock()

	// Проверяем наличие готовых задач
	log.Printf("=== GRPC SERVER: Количество задач в системе: %d, в очереди готовых: %d", 
		len(Manager.Tasks), len(Manager.ReadyTasks))

	// Обновляем список готовых задач перед выдачей
	log.Printf("=== GRPC SERVER: Вызываем UpdateReadyTasksList() для обновления списка готовых задач")
	Manager.UpdateReadyTasksList()
	log.Printf("=== GRPC SERVER: После обновления в очереди готовых: %d", len(Manager.ReadyTasks))

	// Если нет готовых задач, возвращаем пустую задачу
	if len(Manager.ReadyTasks) == 0 {
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

	// Берём первую задачу из очереди
	task := Manager.ReadyTasks[0]
	log.Printf("=== GRPC SERVER: Выбрана задача #%d для выдачи агенту: %s %s %s", 
		task.ID, task.Arg1, task.Operation, task.Arg2)
	
	// Удаляем её из очереди
	if len(Manager.ReadyTasks) > 1 {
		Manager.ReadyTasks = Manager.ReadyTasks[1:]
	} else {
		Manager.ReadyTasks = []models.Task{}
	}

	// Отмечаем задачу как обрабатываемую и сохраняем время начала обработки
	Manager.ProcessingTasks[task.ID] = true
	Manager.TaskProcessingStartTime[task.ID] = time.Now()

	log.Printf("=== GRPC SERVER: Возвращаем задачу #%d агенту #%d: %s %s %s, ExprID=%s", 
		task.ID, req.AgentID, task.Arg1, task.Operation, task.Arg2, task.ExpressionID)

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

	// Прямое сохранение результата в менеджере задач
	Manager.mu.Lock()
	
	// Сохраняем результат в памяти
	Manager.Results[int(result.ID)] = result.Result
	log.Printf("=== GRPC SERVER: Результат задачи #%d сохранен в памяти: %f", result.ID, result.Result)
	
	// Удаляем задачу из списка задач в обработке
	delete(Manager.ProcessingTasks, int(result.ID))
	delete(Manager.TaskProcessingStartTime, int(result.ID))
	
	// Обновляем список готовых задач
	Manager.UpdateReadyTasksList()
	
	// Получаем ID выражения
	exprID := result.ExpressionID
	Manager.mu.Unlock()

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

	log.Printf("=== GRPC SERVER: Результат задачи #%d успешно обработан, очередь готовых задач обновлена", result.ID)
	
	return &calculator.SubmitResultResponse{
		Success: true,
		Message: "результат обработан",
	}, nil
}
