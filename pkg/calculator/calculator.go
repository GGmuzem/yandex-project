package calculator

import (
	"context"

	"github.com/GGmuzem/yandex-project/pkg/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Интерфейс для CalculatorClient
type CalculatorClient interface {
	GetTask(ctx context.Context, in *GetTaskRequest, opts ...interface{}) (*Task, error)
	SubmitResult(ctx context.Context, in *TaskResult, opts ...interface{}) (*SubmitResultResponse, error)
}

// Интерфейс для CalculatorServer
type CalculatorServer interface {
	GetTask(ctx context.Context, in *GetTaskRequest) (*Task, error)
	SubmitResult(ctx context.Context, in *TaskResult) (*SubmitResultResponse, error)
}

// Базовая реализация CalculatorServer
type UnimplementedCalculatorServer struct{}

// Стаб для GetTask
func (s *UnimplementedCalculatorServer) GetTask(ctx context.Context, in *GetTaskRequest) (*Task, error) {
	return nil, status.Errorf(codes.Unimplemented, "метод GetTask не реализован")
}

// Стаб для SubmitResult
func (s *UnimplementedCalculatorServer) SubmitResult(ctx context.Context, in *TaskResult) (*SubmitResultResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "метод SubmitResult не реализован")
}

// RegisterCalculatorServer регистрирует сервер Calculator в gRPC
func RegisterCalculatorServer(s *grpc.Server, srv CalculatorServer) {
	s.RegisterService(&_Calculator_serviceDesc, srv)
}

var _Calculator_serviceDesc = grpc.ServiceDesc{
	ServiceName: "calculator.Calculator",
	HandlerType: (*CalculatorServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetTask",
			Handler:    _Calculator_GetTask_Handler,
		},
		{
			MethodName: "SubmitResult",
			Handler:    _Calculator_SubmitResult_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "calculator.proto",
}

// Обработчик GetTask
func _Calculator_GetTask_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetTaskRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CalculatorServer).GetTask(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/calculator.Calculator/GetTask",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CalculatorServer).GetTask(ctx, req.(*GetTaskRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Обработчик SubmitResult
func _Calculator_SubmitResult_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TaskResult)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CalculatorServer).SubmitResult(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/calculator.Calculator/SubmitResult",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CalculatorServer).SubmitResult(ctx, req.(*TaskResult))
	}
	return interceptor(ctx, in, info, handler)
}

// NewCalculatorClient создает нового клиента для сервиса Calculator
func NewCalculatorClient(cc interface{}) CalculatorClient {
	return &calculatorClient{cc}
}

// Реализация клиента
type calculatorClient struct {
	cc interface{}
}

// GetTask вызывает GetTask у сервера
func (c *calculatorClient) GetTask(ctx context.Context, in *GetTaskRequest, opts ...interface{}) (*Task, error) {
	// Заглушка для компиляции
	return &Task{}, nil
}

// SubmitResult вызывает SubmitResult у сервера
func (c *calculatorClient) SubmitResult(ctx context.Context, in *TaskResult, opts ...interface{}) (*SubmitResultResponse, error) {
	// Заглушка для компиляции
	return &SubmitResultResponse{}, nil
}

// GetTaskRequest запрос на получение задачи
type GetTaskRequest struct {
	AgentID int32 `json:"agent_id"`
}

// Task структура задачи
type Task struct {
	ID            int32  `json:"id"`
	Arg1          string `json:"arg1"`
	Arg2          string `json:"arg2"`
	Operation     string `json:"operation"`
	OperationTime int32  `json:"operation_time"`
	ExpressionID  string `json:"expression_id,omitempty"`
}

// TaskResult результат выполнения задачи
type TaskResult struct {
	ID           int32   `json:"id"`
	Result       float64 `json:"result"`
	ExpressionID string  `json:"expression_id,omitempty"`
}

// SubmitResultResponse ответ на отправку результата
type SubmitResultResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ConvertTaskToGRPC конвертирует модель Task в gRPC формат
func ConvertTaskToGRPC(task models.Task) *Task {
	return &Task{
		ID:            int32(task.ID),
		Arg1:          task.Arg1,
		Arg2:          task.Arg2,
		Operation:     task.Operation,
		OperationTime: int32(task.OperationTime),
		ExpressionID:  task.ExpressionID,
	}
}

// ConvertGRPCToTask конвертирует gRPC Task в модель Task
func ConvertGRPCToTask(task *Task) models.Task {
	return models.Task{
		ID:            int(task.ID),
		Arg1:          task.Arg1,
		Arg2:          task.Arg2,
		Operation:     task.Operation,
		OperationTime: int(task.OperationTime),
		ExpressionID:  task.ExpressionID,
	}
}

// ConvertTaskResultToGRPC конвертирует TaskResult в gRPC формат
func ConvertTaskResultToGRPC(taskResult models.TaskResult, exprID string) *TaskResult {
	return &TaskResult{
		ID:           int32(taskResult.ID),
		Result:       taskResult.Result,
		ExpressionID: exprID,
	}
}
