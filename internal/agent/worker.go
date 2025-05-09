package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/GGmuzem/yandex-project/pkg/models"
)

// getTaskResult получает результат задачи от оркестратора
func getTaskResult(taskID int) (float64, error) {
	// В идеале здесь должен быть запрос к оркестратору через HTTP или gRPC
	// Но так как оркестратор уже проверил, что задача готова к выполнению,
	// и агент получает только готовые задачи, мы можем просто использовать
	// значение, которое уже должно быть в аргументе
	
	// Получаем адрес HTTP сервера из переменной окружения
	httpServer := os.Getenv("HTTP_SERVER")
	if httpServer == "" {
		httpServer = "localhost:8080"
	}
	
	// Формируем URL для запроса результата задачи
	url := fmt.Sprintf("http://%s/internal/task/result/%d", httpServer, taskID)
	
	// Отправляем GET запрос
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Ошибка при запросе результата задачи #%d: %v", taskID, err)
		return 0, err
	}
	defer resp.Body.Close()
	
	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		log.Printf("Ошибка при запросе результата задачи #%d: статус %d", taskID, resp.StatusCode)
		return 0, fmt.Errorf("ошибка при запросе результата: статус %d", resp.StatusCode)
	}
	
	// Читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Ошибка при чтении ответа: %v", err)
		return 0, err
	}
	
	// Парсим JSON ответ
	var result struct {
		Value float64 `json:"value"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("Ошибка при разборе JSON: %v", err)
		return 0, err
	}
	
	return result.Value, nil
}

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

	// Настройка HTTP-сервера для запросов задач
	httpServer := os.Getenv("HTTP_SERVER")
	if httpServer == "" {
		httpServer = "localhost:8080"
	}
	taskURL := "http://" + httpServer + "/internal/task"

	// Интервал между запросами при отсутствии задач
	retryInterval := 100 * time.Millisecond
	// Максимальное количество попыток отправки результата
	maxRetries := 5

	log.Printf("Воркер %d: запущен", id)

	for {
		// Запрашиваем задачу от оркестратора
		req, err := http.NewRequest("GET", taskURL, nil)
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
			resp, err = client.Post(taskURL, "application/json", bytes.NewBuffer(resultData))

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
	// Проверяем на пустые аргументы
	if t.Arg1 == "" || t.Arg2 == "" {
		log.Printf("Ошибка: пустые аргументы в задаче #%d: '%s', '%s'", t.ID, t.Arg1, t.Arg2)
		return 0
	}

	log.Printf("Задача #%d: обработка аргументов: '%s' %s '%s'", t.ID, t.Arg1, t.Operation, t.Arg2)

	// Проверка, является ли аргумент ссылкой на результат другой задачи
	var a, b float64

	// Обработка первого аргумента
	if strings.HasPrefix(t.Arg1, "result") {
		// Это ссылка на результат другой задачи
		// Результаты должны быть предварительно подготовлены оркестратором
		// Агент получает задачу только когда она готова к выполнению
		// Поэтому мы просто извлекаем числовое значение из аргумента
		log.Printf("Задача #%d: аргумент 1 '%s' является ссылкой на результат другой задачи", t.ID, t.Arg1)
		
		// Извлекаем ID задачи из ссылки (например, из "result1" получаем "1")
		resultID := strings.TrimPrefix(t.Arg1, "result")
		
		// Преобразуем ID в число
		resultTaskID, err := strconv.Atoi(resultID)
		if err != nil {
			log.Printf("Ошибка при извлечении ID задачи из ссылки '%s': %v", t.Arg1, err)
			return 0
		}
		
		// Получаем результат предыдущей задачи от оркестратора
		// Это должно быть сделано через HTTP или gRPC запрос к оркестратору
		// Но так как оркестратор уже проверил, что задача готова к выполнению,
		// и агент получает только готовые задачи, мы можем просто использовать
		// числовое значение, переданное в аргументе
		
		// Запрашиваем результат у оркестратора
		result, err := getTaskResult(resultTaskID)
		if err != nil {
			log.Printf("Ошибка при получении результата задачи #%d: %v", resultTaskID, err)
			return 0
		}
		
		a = result
		log.Printf("Задача #%d: получен результат задачи #%d: %f", t.ID, resultTaskID, a)
	} else {
		// Обычное числовое значение
		var errA error
		a, errA = strconv.ParseFloat(t.Arg1, 64)
		if errA != nil {
			log.Printf("Невозможно преобразовать аргумент 1 '%s' в число для задачи #%d: %v", t.Arg1, t.ID, errA)
			return 0
		}
	}

	// Обработка второго аргумента
	if strings.HasPrefix(t.Arg2, "result") {
		// Это ссылка на результат другой задачи
		log.Printf("Задача #%d: аргумент 2 '%s' является ссылкой на результат другой задачи", t.ID, t.Arg2)
		
		// Извлекаем ID задачи из ссылки
		resultID := strings.TrimPrefix(t.Arg2, "result")
		
		// Преобразуем ID в число
		resultTaskID, err := strconv.Atoi(resultID)
		if err != nil {
			log.Printf("Ошибка при извлечении ID задачи из ссылки '%s': %v", t.Arg2, err)
			return 0
		}
		
		// Получаем результат предыдущей задачи
		result, err := getTaskResult(resultTaskID)
		if err != nil {
			log.Printf("Ошибка при получении результата задачи #%d: %v", resultTaskID, err)
			return 0
		}
		
		b = result
		log.Printf("Задача #%d: получен результат задачи #%d: %f", t.ID, resultTaskID, b)
	} else {
		// Обычное числовое значение
		var errB error
		b, errB = strconv.ParseFloat(t.Arg2, 64)
		if errB != nil {
			log.Printf("Невозможно преобразовать аргумент 2 '%s' в число для задачи #%d: %v", t.Arg2, t.ID, errB)
			return 0
		}
	}

	log.Printf("Задача #%d: преобразованные аргументы: %f %s %f", t.ID, a, t.Operation, b)

	// Выполняем операцию
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
		log.Printf("Неизвестная операция в задаче #%d: %s", t.ID, t.Operation)
	}
	return 0
}
