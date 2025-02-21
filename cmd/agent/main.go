package main

import (
    "log"
    "github.com/GGmuzem/yandex-project/internal/agent"
)

func main() {
    log.Println("Agent starting...")
    agent.StartWorker()
    select {} // Бесконечный цикл
}