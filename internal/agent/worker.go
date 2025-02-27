package agent

import (
	"bytes"
	"encoding/json"
	"io"

	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/GGmuzem/yandex-project/pkg/models"
)

// StartWorker запускает агент с несколькими воркерами
func StartWorker() {
	// Получаем количество вычислительных мощностей из переменной окружения
	power, err := strconv.Atoi(os.Getenv("COMPUTING_POWER"))
	if err != nil || power <= 0 {
		power = 10 // По умолчанию 1 воркер
		log.Printf("COMPUTING_POWER не указано или некорректно, используем значение по умолчанию: %d", power)
	}

	log.Printf("Запуск агента с %d воркерами", power)

	// Запускаем необходимое количество горутин
	for i := 0; i < power; i++ {
		go workerLoop(i)
	}
}

// workerLoop непрерывно опрашивает оркестратор на наличие новых задач и выполняет их
func workerLoop(id int) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Интервал между запросами при отсутствии задач
	retryInterval := 100 * time.Millisecond
	// Максимальное количество попыток отправки результата
	maxRetries := 5

	log.Printf("Воркер %d: запущен", id)

	for {
		// Запрашиваем задачу от оркестратора
		req, err := http.NewRequest("GET", "http://localhost:8080/internal/task", nil)
		if err != nil {
			log.Printf("Воркер %d: ошибка создания запроса: %v", id, err)
			time.Sleep(retryInterval)
			continue
		}

		resp, err := client.Do(req)

		// Обрабатываем ошибки и случай отсутствия задач
		if err != nil {
			log.Printf("Воркер %d: ошибка соединения с оркестратором: %v", id, err)
			time.Sleep(retryInterval)
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			// Если нет задач, повторяем через небольшой интервал
			resp.Body.Close()
			time.Sleep(retryInterval)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("Воркер %d: неожиданный статус от оркестратора: %d", id, resp.StatusCode)
			resp.Body.Close()
			time.Sleep(retryInterval)
			continue
		}

		// Декодируем ответ с задачей
		var taskResp struct {
			Task models.Task `json:"task"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&taskResp); err != nil {
			log.Printf("Воркер %d: ошибка декодирования ответа: %v", id, err)
			resp.Body.Close()
			time.Sleep(retryInterval)
			continue
		}
		resp.Body.Close()

		task := taskResp.Task
		log.Printf("Воркер %d: получена задача #%d: %s %s %s", id, task.ID, task.Arg1, task.Operation, task.Arg2)

		// Вычисляем результат
		result := computeTask(task)

		// Имитируем длительное время вычисления
		log.Printf("Воркер %d: выполняется задача #%d (%d мс)...", id, task.ID, task.OperationTime)
		time.Sleep(time.Duration(task.OperationTime) * time.Millisecond)

		// Подготавливаем данные результата
		resultData, err := json.Marshal(models.TaskResult{
			ID:     task.ID,
			Result: result,
		})

		if err != nil {
			log.Printf("Воркер %d: ошибка маршалинга результата: %v", id, err)
			continue
		}

		log.Printf("Воркер %d: отправляю результат задачи #%d: %s", id, task.ID, string(resultData))

		// Механизм повторных попыток отправки результата
		retryCount := 0
		success := false

		for retryCount < maxRetries && !success {
			if retryCount > 0 {
				log.Printf("Воркер %d: повторная попытка #%d отправки результата задачи #%d", id, retryCount, task.ID)
				time.Sleep(retryInterval * time.Duration(retryCount)) // Увеличиваем интервал с каждой попыткой
			}

			// Создаем новый буфер для каждой попытки
			resp, err = client.Post("http://localhost:8080/internal/task", "application/json", bytes.NewBuffer(resultData))

			if err != nil {
				log.Printf("Воркер %d: ошибка отправки результата (попытка %d): %v", id, retryCount+1, err)
				retryCount++
				continue
			}

			respBody, _ := io.ReadAll(resp.Body)
			log.Printf("Воркер %d: ответ от оркестратора (попытка %d): %s (статус %d)",
				id, retryCount+1, string(respBody), resp.StatusCode)

			if resp.StatusCode == http.StatusOK {
				success = true
				log.Printf("Воркер %d: задача #%d успешно завершена, результат: %f", id, task.ID, result)
			} else {
				log.Printf("Воркер %d: оркестратор вернул ошибку при отправке результата: %d", id, resp.StatusCode)
			}

			resp.Body.Close()
			retryCount++
		}

		if !success {
			log.Printf("Воркер %d: не удалось отправить результат задачи #%d после %d попыток",
				id, task.ID, maxRetries)
		}
	}
}

// computeTask выполняет арифметическую операцию
func computeTask(t models.Task) float64 {
	a, errA := strconv.ParseFloat(t.Arg1, 64)
	b, errB := strconv.ParseFloat(t.Arg2, 64)

	// Если один из аргументов не является числом, проверяем, может быть это результат предыдущей операции
	if errA != nil || errB != nil {
		log.Printf("Предупреждение: аргументы не являются числами: %s, %s", t.Arg1, t.Arg2)
		return 0
	}

	switch t.Operation {
	case "+":
		return a + b
	case "-":
		return a - b
	case "*":
		return a * b
	case "/":
		if b == 0 {
			log.Printf("Ошибка: деление на ноль в задаче #%d", t.ID)
			return 0
		}
		return a / b
	default:
		log.Printf("Неизвестная операция: %s", t.Operation)
	}
	return 0
}
