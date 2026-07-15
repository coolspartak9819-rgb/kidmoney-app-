package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/coolspartak9819-rgb/kidmoney-app/internal/model"
	"github.com/coolspartak9819-rgb/kidmoney-app/internal/repository"

	"github.com/gin-gonic/gin"
)

// GetBalance возвращает текущий баланс пользователя.
func GetBalance(c *gin.Context) {
	userID, _ := c.Get("userID")

	var balance float64
	err := repository.DB.QueryRow("SELECT balance FROM users WHERE id = ?", userID).Scan(&balance)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении баланса"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"balance": balance})
}

// GetTransactions возвращает историю транзакций пользователя.
func GetTransactions(c *gin.Context) {
	userID, _ := c.Get("userID")

	rows, err := repository.DB.Query("SELECT id, user_id, amount, description, type, created_at FROM transactions WHERE user_id = ? ORDER BY created_at DESC", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении транзакций"})
		return
	}
	defer rows.Close()

	transactions := []model.Transaction{}
	for rows.Next() {
		var t model.Transaction
		if err := rows.Scan(&t.ID, &t.UserID, &t.Amount, &t.Description, &t.Type, &t.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при сканировании транзакции"})
			return
		}
		transactions = append(transactions, t)
	}

	c.JSON(http.StatusOK, transactions)
}

type TransactionInput struct {
	Amount      float64 `json:"amount" binding:"required"`
	Description string  `json:"description" binding:"required"`
}

// Purchase обрабатывает списание средств (покупку).
func Purchase(c *gin.Context) {
	userID, _ := c.Get("userID")

	var input TransactionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Сумма должна быть положительной"})
		return
	}

	tx, err := repository.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при старте транзакции"})
		return
	}

	// Проверяем баланс
	var currentBalance float64
	err = tx.QueryRow("SELECT balance FROM users WHERE id = ?", userID).Scan(&currentBalance)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}

	if currentBalance < input.Amount {
		tx.Rollback()
		c.JSON(http.StatusForbidden, gin.H{"error": "Недостаточно средств"})
		return
	}

	// Списываем средства
	newBalance := currentBalance - input.Amount
	_, err = tx.Exec("UPDATE users SET balance = ? WHERE id = ?", newBalance, userID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при обновлении баланса"})
		return
	}

	// Записываем транзакцию
	_, err = tx.Exec("INSERT INTO transactions(user_id, amount, description, type, created_at) VALUES(?, ?, ?, ?, ?)",
		userID, input.Amount, input.Description, "expense", time.Now())
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при записи транзакции"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при подтверждении транзакции"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Покупка совершена успешно", "new_balance": newBalance})
}

// Deduction обрабатывает удержание средств (штраф, доступно только для 'parent').
func Deduction(c *gin.Context) {
	userRole, _ := c.Get("userRole")
	if userRole != "parent" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещен: только родители могут делать удержания"})
		return
	}

	var input TransactionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// ID ребенка получаем из URL
	childIdStr := c.Param("userId")
	childId, err := strconv.ParseInt(childIdStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID пользователя"})
		return
	}

	tx, err := repository.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при старте транзакции"})
		return
	}

	// Списываем средства
	_, err = tx.Exec("UPDATE users SET balance = balance - ? WHERE id = ?", input.Amount, childId)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при обновлении баланса"})
		return
	}

	// Записываем транзакцию
	_, err = tx.Exec("INSERT INTO transactions(user_id, amount, description, type, created_at) VALUES(?, ?, ?, ?, ?)",
		childId, input.Amount, input.Description, "expense", time.Now())
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при записи транзакции"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при подтверждении транзакции"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Удержание успешно применено"})
}
