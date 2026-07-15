package handler

import (
	"database/sql"
	"net/http"
	"time"

	"kidmoney-app/internal/middleware"
	"kidmoney-app/internal/model"
	"kidmoney-app/internal/repository"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type RegisterInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role" binding:"required"` // "parent" или "child"
}

// Register обрабатывает регистрацию нового пользователя.
func Register(c *gin.Context) {
	var input RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Проверка роли
	if input.Role != "parent" && input.Role != "child" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Роль должна быть 'parent' или 'child'"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при хешировании пароля"})
		return
	}

	user := model.User{
		Username:  input.Username,
		Password:  string(hashedPassword),
		Role:      input.Role,
		Balance:   0,
		CreatedAt: time.Now(),
	}

	stmt, err := repository.DB.Prepare("INSERT INTO users(username, password, role, created_at) VALUES(?, ?, ?, ?)")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка подготовки запроса"})
		return
	}
	defer stmt.Close()

	res, err := stmt.Exec(user.Username, user.Password, user.Role, user.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Пользователь с таким именем уже существует"})
		return
	}

	id, _ := res.LastInsertId()
	user.ID = id

	c.JSON(http.StatusCreated, gin.H{"message": "Пользователь успешно зарегистрирован", "user_id": user.ID})
}

type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login обрабатывает аутентификацию пользователя.
func Login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user model.User
	err := repository.DB.QueryRow("SELECT id, username, password, role FROM users WHERE username = ?", input.Username).Scan(&user.ID, &user.Username, &user.Password, &user.Role)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверное имя пользователя или пароль"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сервера"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверное имя пользователя или пароль"})
		return
	}

	token, err := middleware.GenerateJWT(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при создании токена"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}
