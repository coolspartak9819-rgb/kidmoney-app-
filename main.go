package main

import (
	"log"

	"github.com/coolspartak9819-rgb/kidmoney-app/internal/handler"
	"github.com/coolspartak9819-rgb/kidmoney-app/internal/middleware"
	"github.com/coolspartak9819-rgb/kidmoney-app/internal/repository"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Инициализация базы данных
	repository.InitDB("./kidmoney.db")

	// Создание роутера Gin
	r := gin.Default()

	// Настройка CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // В продакшене укажите конкретный домен
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Группа публичных роутов
	public := r.Group("/api")
	{
		public.POST("/register", middleware.Register)
		public.POST("/login", middleware.Login)
	}

	// Группа защищенных роутов
	protected := r.Group("/api")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.GET("/tasks", handler.GetTasks)
		protected.POST("/tasks", handler.CreateTask)
		protected.POST("/tasks/complete/:id", handler.CompleteTask)

		protected.GET("/balance", handler.GetBalance)
		protected.GET("/transactions", handler.GetTransactions)
		protected.POST("/purchase", handler.Purchase)
		protected.POST("/deduction/:userId", handler.Deduction)
	}

	log.Println("Сервер запускается на порту :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Не удалось запустить сервер: %v", err)
	}
}
