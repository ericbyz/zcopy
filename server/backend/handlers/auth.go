package handlers

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"zcopy-server-backend/config"
	"zcopy-server-backend/database"
	"zcopy-server-backend/middleware"
	"zcopy-server-backend/models"
	"zcopy-server-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

type loginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

func Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请求参数格式错误"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Nickname = strings.TrimSpace(req.Nickname)

	if req.Username == "" || req.Email == "" || len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "用户名、邮箱和密码不合法"})
		return
	}

	if database.DB.UserExists(req.Username, req.Email) {
		c.JSON(http.StatusConflict, gin.H{"message": "用户名或邮箱已存在"})
		return
	}

	user := models.User{
		Username: req.Username,
		Email:    req.Email,
		Password: utils.HashPassword(req.Password),
		Nickname: req.Nickname,
	}

	if err := database.DB.CreateUser(&user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "创建用户失败"})
		return
	}

	if err := utils.EnsureDirectoryExists(userRoot(user.ID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "创建用户空间失败"})
		return
	}

	token, err := issueToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "生成登录凭证失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "注册成功",
		"token":   token,
		"user":    sanitizeUser(user),
	})
}

func Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请求参数格式错误"})
		return
	}

	req.Account = strings.TrimSpace(req.Account)
	if req.Account == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "账号或密码不能为空"})
		return
	}

	user, err := database.DB.FindUserByAccount(strings.ToLower(req.Account))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "账号或密码错误"})
		return
	}

	if !utils.ComparePasswords(user.Password, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "账号或密码错误"})
		return
	}

	token, err := issueToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "生成登录凭证失败"})
		return
	}

	if err := utils.EnsureDirectoryExists(userRoot(user.ID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "初始化用户空间失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"token":   token,
		"user":    sanitizeUser(user),
	})
}

func Me(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": sanitizeUser(user),
	})
}

func issueToken(userID uint) (string, error) {
	expiresAt := time.Now().Add(time.Duration(config.AppConfig.Auth.TokenExpireHours) * time.Hour)
	claims := middleware.AuthClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   "zcopy-user",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.Auth.SecretKey))
}

func sanitizeUser(user models.User) gin.H {
	return gin.H{
		"id":        user.ID,
		"username":  user.Username,
		"email":     user.Email,
		"nickname":  user.Nickname,
		"avatar":    user.Avatar,
		"createdAt": user.CreatedAt,
	}
}

func userRoot(userID uint) string {
	return filepath.Join(config.AppConfig.Storage.RootDir, "user-"+utils.UintToString(userID))
}
