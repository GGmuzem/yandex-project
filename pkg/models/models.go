package models

type Expression struct {
    ID     string  `json:"id"`
    Status string  `json:"status"`
    Result float64 `json:"result,omitempty"`
}

type Task struct {
    ID            int    `json:"id"`
    Arg1          string `json:"arg1"`
    Arg2          string `json:"arg2"`
    Operation     string `json:"operation"`
    OperationTime int    `json:"operation_time"`
}

type TaskResult struct {
    ID     int     `json:"id"`
    Result float64 `json:"result"`
}