package agent

import (
    "bytes"
    "encoding/json"
    "log"
    "net/http"
    "os"
    "strconv"
    "time"
    "github.com/GGmuzem/yandex-project/pkg/models"
)

func StartWorker() {
    power, _ := strconv.Atoi(os.Getenv("COMPUTING_POWER"))
    if power <= 0 {
        power = 1
    }

    for i := 0; i < power; i++ {
        go workerLoop(i)
    }
}

func workerLoop(id int) {
    client := &http.Client{}
    for {
        resp, err := client.Get("http://localhost:8080/internal/task")
        if err != nil || resp.StatusCode == http.StatusNotFound {
            time.Sleep(100 * time.Millisecond)
            continue
        }

        var taskResp struct {
            Task models.Task `json:"task"`
        }
        json.NewDecoder(resp.Body).Decode(&taskResp)
        resp.Body.Close()

        task := taskResp.Task
        result := computeTask(task)
        time.Sleep(time.Duration(task.OperationTime) * time.Millisecond)

        resultData, _ := json.Marshal(models.TaskResult{
            ID:     task.ID,
            Result: result,
        })
        _, err = client.Post("http://localhost:8080/internal/task", "application/json", bytes.NewBuffer(resultData))
        if err != nil {
            log.Printf("Worker %d: failed to send result: %v", id, err)
        }
    }
}

func computeTask(t models.Task) float64 {
    a, _ := strconv.ParseFloat(t.Arg1, 64)
    b, _ := strconv.ParseFloat(t.Arg2, 64)
    switch t.Operation {
    case "+":
        return a + b
    case "-":
        return a - b
    case "*":
        return a * b
    case "/":
        if b != 0 {
            return a / b
        }
    }
    return 0
}