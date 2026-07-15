package handler

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/coolspartak9819-rgb/kidmoney-app/internal/model"
	"github.com/coolspartak9819-rgb/kidmoney-app/internal/repository"

	"github.com/gin-gonic/gin"
)

// GetTasks возвращает список всех активных задач.
func GetTasks(c *gin.Context) {
	rows, err := repository.DB.Query("SELECT id, title, description, amount, type, is_active, created_at FROM tasks WHERE is_active = TRUE")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении задач"})
		return
	}
	defer rows.Close()

	tasks := []model.Task{}
	for rows.Next() {
		var task model.Task
		if err := rows.Scan(&task.ID, &task.Title, &task.Description, &task.Amount, &task.Type, &task.IsActive, &task.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при сканировании задачи"})
			return
		}
		tasks = append(tasks, task)
	}

	c.JSON(http.StatusOK, tasks)
}

type CreateTaskInput struct {
	Title       string  `json:"title" binding:"required"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount" binding:"required"`
	Type        string  `json:"type" binding:"required"`
}

// CreateTask создает новую задачу (доступно только для 'parent').
func CreateTask(c *gin.Context) {
	userRole, _ := c.Get("userRole")
	if userRole != "parent" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещен: только родители могут создавать задачи"})
		return
	}

	var input CreateTaskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task := model.Task{
		Title:       input.Title,
		Description: input.Description,
		Amount:      input.Amount,
		Type:        input.Type,
		IsActive:    true,
		CreatedAt:   time.Now(),
	}

	stmt, err := repository.DB.Prepare("INSERT INTO tasks(title, description, amount, type, is_active, created_at) VALUES(?, ?, ?, ?, ?, ?)")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка подготовки запроса"})
		return
	}
	defer stmt.Close()

	res, err := stmt.Exec(task.Title, task.Description, task.Amount, task.Type, task.IsActive, task.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при создании задачи"})
		return
	}

	id, _ := res.LastInsertId()
	task.ID = id

	c.JSON(http.StatusCreated, task)
}

type CompleteTaskInput struct {
	TaskID int64 `json:"task_id" binding:"required"`
}

// CompleteTask помечает задачу как выполненную и начисляет средства.
func CompleteTask(c *gin.Context) {
	userID, _ := c.Get("userID")
	userRole, _ := c.Get("userRole")

	if userRole != "child" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Только ребенок может выполнять задачи."})
		return
	}

	var input CompleteTaskInput
	err := c.ShouldBindJSON(&input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID задачи"})
		return
	}

	tx, err := repository.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при старте транзакции"})
		return
	}

	// Находим задачу и проверяем, активна ли она
	var task model.Task
	err = tx.QueryRow("SELECT id, amount, title FROM tasks WHERE id = ? AND is_active = TRUE", input.TaskID).Scan(&task.ID, &task.Amount, &task.Title)
	if err != nil {
		tx.Rollback()
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Задача не найдена или уже неактивна"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при поиске задачи"})
		return
	}

	// Деактивируем задачу
	_, err = tx.Exec("UPDATE tasks SET is_active = FALSE WHERE id = ?", input.TaskID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при обновлении задачи"})
		return
	}

	// Начисляем средства на баланс
	_, err = tx.Exec("UPDATE users SET balance = balance + ? WHERE id = ?", task.Amount, userID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при начислении средств"})
		return
	}

	// Записываем транзакцию
	_, err = tx.Exec("INSERT INTO transactions(user_id, amount, description, type, created_at) VALUES(?, ?, ?, ?, ?)",
		userID, task.Amount, "Выполнение задачи: "+task.Title, "income", time.Now())
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при записи транзакции"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при подтверждении транзакции"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Задача успешно выполнена", "amount_added": task.Amount})
}
