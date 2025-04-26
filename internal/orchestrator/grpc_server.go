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

	// Блокируем доступ к менеджеру задач
	Manager.mu.Lock()
	defer Manager.mu.Unlock()

	// Логируем состояние очереди готовых задач и всех задач
	log.Printf("=== ОТЛАДКА SERVER: Всего задач в менеджере: %d", len(Manager.Tasks))
	log.Printf("=== ОТЛАДКА SERVER: Длина очереди готовых задач: %d", len(Manager.ReadyTasks))

	// Проверяем содержимое карты задач
	for id, task := range Manager.Tasks {
		log.Printf("=== ОТЛАДКА SERVER: В карте задач ID=%d: Arg1='%s', Arg2='%s', Operation='%s', ExpressionID='%s'",
			id, task.Arg1, task.Arg2, task.Operation, task.ExpressionID)
	}

	// Проверяем очередь готовых задач
	for i, task := range Manager.ReadyTasks {
		log.Printf("=== ОТЛАДКА SERVER: Задача #%d в очереди: ID=%d, Arg1='%s', Arg2='%s', Operation='%s', ExpressionID='%s'",
			i, task.ID, task.Arg1, task.Arg2, task.Operation, task.ExpressionID)
	}

	// Проверяем задачи в обработке
	log.Printf("=== ОТЛАДКА SERVER: Задачи в обработке:")
	for taskID, agentID := range Manager.ProcessingTasks {
		log.Printf("=== ОТЛАДКА SERVER: Задача #%d обрабатывается агентом: %v", taskID, agentID)
	}

	// Если нет готовых задач, возвращаем пустую задачу
	if len(Manager.ReadyTasks) == 0 {
		log.Printf("=== ОТЛАДКА SERVER: Нет готовых задач для агента ID=%d", req.AgentID)
		return &calculator.Task{}, nil
	}

	// Проверим, есть ли задачи в состоянии готовности
	readyTaskIndex := -1
	for i, task := range Manager.ReadyTasks {
		processingStatus, isProcessing := Manager.ProcessingTasks[task.ID]
		log.Printf("=== ОТЛАДКА SERVER: Проверка задачи #%d: в обработке=%v, статус=%v",
			task.ID, isProcessing, processingStatus)

		if task.ID > 0 && !isProcessing {
			readyTaskIndex = i
			log.Printf("=== ОТЛАДКА SERVER: Найдена готовая задача #%d: %s %s %s",
				task.ID, task.Arg1, task.Operation, task.Arg2)
			break
		}
	}

	// Если нет готовых задач, возвращаем пустую задачу
	if readyTaskIndex == -1 {
		log.Printf("=== ОТЛАДКА SERVER: Все задачи уже в обработке для агента ID=%d", req.AgentID)
		return &calculator.Task{}, nil
	}

	// Получаем первую задачу из очереди и удаляем ее из очереди
	task := Manager.ReadyTasks[readyTaskIndex]

	// Если задача не в начале очереди, меняем порядок и усекаем очередь
	if readyTaskIndex < len(Manager.ReadyTasks)-1 {
		// Перемещаем элемент на последнее место и удаляем
		Manager.ReadyTasks[readyTaskIndex] = Manager.ReadyTasks[len(Manager.ReadyTasks)-1]
	}
	Manager.ReadyTasks = Manager.ReadyTasks[:len(Manager.ReadyTasks)-1]

	log.Printf("=== ОТЛАДКА SERVER: Взята задача ID=%d из очереди (индекс %d), осталось задач: %d",
		task.ID, readyTaskIndex, len(Manager.ReadyTasks))

	// Отмечаем задачу как обрабатываемую
	Manager.ProcessingTasks[task.ID] = true
	log.Printf("=== ОТЛАДКА SERVER: Задача ID=%d помечена как обрабатываемая агентом ID=%d", task.ID, req.AgentID)

	// Сохраняем задачу в карту задач, если она еще не там
	if _, exists := Manager.Tasks[task.ID]; !exists {
		log.Printf("=== ОТЛАДКА SERVER: Задача ID=%d не найдена в карте задач, добавляем", task.ID)
		taskCopy := task
		Manager.Tasks[task.ID] = &taskCopy
	}

	// Преобразуем задачу в protobuf-формат
	taskProto := &calculator.Task{
		ID:            int32(task.ID),
		Arg1:          task.Arg1,
		Arg2:          task.Arg2,
		Operation:     task.Operation,
		ExpressionID:  task.ExpressionID,
		OperationTime: int32(task.OperationTime),
	}

	log.Printf("=== ОТЛАДКА SERVER: Отправка задачи агенту: ID=%d, Arg1='%s', Arg2='%s', Operation='%s', ExpressionID='%s'",
		taskProto.ID, taskProto.Arg1, taskProto.Arg2, taskProto.Operation, taskProto.ExpressionID)

	return taskProto, nil
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
