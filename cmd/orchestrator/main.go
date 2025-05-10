package main

import (
    "log"
    "github.com/GGmuzem/yandex-project/internal/orchestrator"
)

func main() {
    // Инициализируем глобальный экземпляр менеджера выражений
    log.Println("Инициализация менеджера выражений")
    
    // Запускаем HTTP-сервер с обработкой маршрутов через gorilla/mux
    log.Println("Запуск HTTP-сервера на порту :8081")
    // Вызываем функцию без параметров, так как порт теперь задаётся внутри функции
    orchestrator.StartServer()
}