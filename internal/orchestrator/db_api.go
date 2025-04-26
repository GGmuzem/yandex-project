package orchestrator

import (
	"log"
	"time"

	"github.com/GGmuzem/yandex-project/internal/database"
)

// DBOrchestrator оркестратор с поддержкой базы данных
type DBOrchestrator struct {
	DB database.Database
}

// NewDBOrchestrator создает новый экземпляр оркестратора с БД
func NewDBOrchestrator(db database.Database) *DBOrchestrator {
	return &DBOrchestrator{
		DB: db,
	}
}

// UpdateExpressionStatusInDB обновляет статус выражения в БД
func (o *DBOrchestrator) UpdateExpressionStatusInDB(id string, status string, result float64) error {
	return o.DB.UpdateExpressionStatus(id, status, result)
}

// LoadExpressionsFromDB загружает выражения из БД в память
func (o *DBOrchestrator) LoadExpressionsFromDB() error {
	// Здесь должна быть функция, которая загружает выражения из БД

	// Пример реализации:
	// exprList, err := o.DB.GetAllExpressions()
	// if err != nil {
	//     return err
	// }
	//
	// mu.Lock()
	// defer mu.Unlock()
	//
	// for _, expr := range exprList {
	//     expressions[expr.ID] = expr
	// }

	return nil
}

// LoadResultsFromDB загружает результаты задач из БД в память
func (o *DBOrchestrator) LoadResultsFromDB(exprID string) error {
	resultsMap, err := o.DB.GetResultsByExprID(exprID)
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()

	for taskID, result := range resultsMap {
		results[taskID] = result
	}

	return nil
}

// SaveResultToDB сохраняет результат задачи в БД
func (o *DBOrchestrator) SaveResultToDB(taskID int, result float64, exprID string) error {
	return o.DB.SaveResult(taskID, result, exprID)
}

// UpdateExpressionsStatusInDB функция для периодического обновления статусов выражений в БД
func (o *DBOrchestrator) UpdateExpressionsStatusInDB() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		mu.Lock()
		log.Println("Обновление статусов выражений в БД...")

		for id, expr := range expressions {
			if err := o.DB.UpdateExpressionStatus(id, expr.Status, expr.Result); err != nil {
				log.Printf("Ошибка обновления статуса выражения %s в БД: %v", id, err)
			}
		}

		mu.Unlock()
	}
}
