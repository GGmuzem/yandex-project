package orchestrator

import (
	"context"
	"fmt"
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

	// Блокируем доступ к менеджеру задач
	Manager.mu.Lock()
	defer Manager.mu.Unlock()

	// Обновляем очередь готовых задач
	log.Printf("=== ОТЛАДКА SERVER: Проверка и обновление готовых задач")
	for taskID, taskPtr := range Manager.Tasks {
		if !Manager.ProcessingTasks[taskID] {
			task := *taskPtr
			if isTaskReady(task) {
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

	// Если нет готовых задач, возвращаем ошибку
	if len(Manager.ReadyTasks) == 0 {
		log.Printf("=== ОТЛАДКА SERVER: Нет готовых задач для агента #%d", req.AgentID)
		return nil, fmt.Errorf("нет доступных задач")
	}

	// Выбираем первую задачу из очереди
	task := Manager.ReadyTasks[0]
	// Удаляем её из очереди
	Manager.ReadyTasks = Manager.ReadyTasks[1:]
	// Помечаем как обрабатываемую
	Manager.ProcessingTasks[task.ID] = true

	log.Printf("=== ОТЛАДКА SERVER: Возвращаем задачу #%d агенту #%d", task.ID, req.AgentID)

	// Преобразуем в protobuf формат
	grpcTask := &calculator.Task{
		ID:            int32(task.ID),
		Arg1:          task.Arg1,
		Arg2:          task.Arg2,
		Operation:     task.Operation,
		OperationTime: int32(task.OperationTime),
		ExpressionID:  task.ExpressionID,
	}

	log.Printf("=== ОТЛАДКА SERVER: Возвращаем задачу агенту #%d: ID=%d, %s %s %s", 
		req.AgentID, grpcTask.ID, grpcTask.Arg1, grpcTask.Operation, grpcTask.Arg2)

	return grpcTask, nil
}

// SubmitResult обрабатывает результат задачи от агента
func (s *CalculatorServer) SubmitResult(ctx context.Context, result *calculator.TaskResult) (*calculator.SubmitResultResponse, error) {
	log.Printf("Получен результат для задачи #%d: %f от агента", result.ID, result.Result)

	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	// Снимаем отметку "в обработке" с задачи
	delete(Manager.ProcessingTasks, int(result.ID))

	// Проверяем, существует ли еще задача
	_, exists := Manager.Tasks[int(result.ID)]
	if !exists {
		// Проверяем, может быть результат для этой задачи уже был получен
		if _, resultExists := Manager.Results[int(result.ID)]; resultExists {
			log.Printf("Результат для задачи #%d уже был получен ранее. Текущий результат: %f. Игнорируем дублирующий результат.",
				result.ID, result.Result)
			return &calculator.SubmitResultResponse{
				Success: false,
				Message: "результат уже получен",
			}, nil
		}

		// Сохраняем результат в базу данных
		if err := s.DB.SaveResult(int(result.ID), result.Result, result.ExpressionID); err != nil {
			log.Printf("Ошибка при сохранении результата в БД для задачи #%d: %v", result.ID, err)
			return &calculator.SubmitResultResponse{
				Success: false,
				Message: "ошибка сохранения результата: " + err.Error(),
			}, nil
		}

		log.Printf("Результат для неактивной задачи #%d сохранен: %f", result.ID, result.Result)

		// Получаем ID выражения, если оно есть
		exprID := result.ExpressionID
		if exprID != "" {
			log.Printf("Неактивная задача #%d принадлежит выражению %s, проверяем зависимости", result.ID, exprID)

			// Обрабатываем зависимости для неактивной задачи
			processTaskDependencies(int(result.ID), result.Result)

			// Обновляем готовые задачи
			updateReadyTasks()

			// Проверяем статусы выражений
			updateExpressions()
		} else {
			log.Printf("Для неактивной задачи #%d не найдено связанное выражение", result.ID)
		}

		return &calculator.SubmitResultResponse{
			Success: true,
			Message: "результат сохранен",
		}, nil
	}

	// Сохраняем результат в базу данных
	if err := s.DB.SaveResult(int(result.ID), result.Result, result.ExpressionID); err != nil {
		log.Printf("Ошибка при сохранении результата в БД для задачи #%d: %v", result.ID, err)
		return &calculator.SubmitResultResponse{
			Success: false,
			Message: "ошибка сохранения результата: " + err.Error(),
		}, nil
	}

	// Сохраняем результат в память
	Manager.Results[int(result.ID)] = result.Result

	// Удаляем выполненную задачу из активных
	delete(Manager.Tasks, int(result.ID))

	// Обрабатываем зависимости
	processTaskDependencies(int(result.ID), result.Result)

	// Обновляем готовые задачи
	updateReadyTasks()

	// Проверяем, остались ли ещё задачи для этого выражения
	tasksRemain := false
	for taskID, id := range Manager.TaskToExpr {
		if id == result.ExpressionID {
			if _, ok := Manager.Tasks[taskID]; ok {
				tasksRemain = true
				log.Printf("Для выражения %s осталась невыполненная задача #%d", result.ExpressionID, taskID)
				break
			}
		}
	}

	if !tasksRemain {
		log.Printf("Для выражения %s не осталось задач, проверяем результат", result.ExpressionID)
	}

	// Обновляем статусы выражений
	updateExpressions()

	log.Printf("Завершена обработка результата для задачи #%d: %f", result.ID, result.Result)

	return &calculator.SubmitResultResponse{
		Success: true,
		Message: "результат обработан",
	}, nil
}
