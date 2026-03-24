package routes

import (
	"net/http"
	"strconv"
	"time"

	"github.com/alldev-run/golang-gin-rpc/api/http-gateway/internal/model"
	"github.com/alldev-run/golang-gin-rpc/pkg/middleware"
	"github.com/alldev-run/golang-gin-rpc/pkg/router"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes 注册用户路由
func RegisterUserRoutes(registry *router.RouteRegistry) {
	// 创建用户路由组，带认证中间件
	userGroup := registry.Group("user", "/api/user")

	// 添加组级别中间件 - 使用框架自动日志记录
	userGroup.Use(authMiddleware())

	// 注册用户路由
	userGroup.POST("", handleUserCreate, "创建用户")
	userGroup.GET("/:id", handleUserGet, "获取用户详情")
	userGroup.PUT("/:id", handleUserUpdate, "更新用户")
	userGroup.DELETE("/:id", handleUserDelete, "删除用户")

	// 用户列表（独立路由，不带认证）
	registry.GET("/api/users", handleUserList, "获取用户列表")
}

// 中间件工厂方法

// authMiddleware 认证中间件
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			apiKey = c.Query("api_key")
		}

		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "API Key required",
				"request_id": middleware.GetRequestID(c),
			})
			c.Abort()
			return
		}

		// 简单验证（实际应查询数据库）
		if apiKey != "test-api-key" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Invalid API Key",
				"request_id": middleware.GetRequestID(c),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// 处理器

func handleUserCreate(c *gin.Context) {
	var req model.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
			"error":   err.Error(),
			"request_id": middleware.GetRequestID(c),
		})
		return
	}

	user := model.User{
		ID:        "1",
		Name:      req.Name,
		Email:     req.Email,
		Age:       req.Age,
		CreatedAt: time.Now(),
	}

	c.JSON(http.StatusCreated, model.UserResponse{
		Success: true,
		Data:    user,
		Message: "用户创建成功",
	})
}

func handleUserGet(c *gin.Context) {
	// 从路径提取ID
	userID := c.Param("id")

	user := model.User{
		ID:    userID,
		Name:  "张三",
		Email: "zhangsan@example.com",
		Age:   25,
	}

	c.JSON(http.StatusOK, model.UserResponse{
		Success: true,
		Data:    user,
		Message: "用户详情",
	})
}

func handleUserUpdate(c *gin.Context) {
	userID := c.Param("id")

	var req model.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
			"error":   err.Error(),
			"request_id": middleware.GetRequestID(c),
		})
		return
	}

	user := model.User{
		ID:    userID,
		Name:  req.Name,
		Email: req.Email,
		Age:   req.Age,
	}

	c.JSON(http.StatusOK, model.UserResponse{
		Success: true,
		Data:    user,
		Message: "用户更新成功",
	})
}

func handleUserDelete(c *gin.Context) {
	userID := c.Param("id")

	c.JSON(http.StatusOK, model.BaseResponse{
		Success: true,
		Message: "用户 " + userID + " 删除成功",
	})
}

func handleUserList(c *gin.Context) {
	page := 1
	pageSize := 10

	if p := c.Query("page"); p != "" {
		page, _ = strconv.Atoi(p)
	}
	if ps := c.Query("page_size"); ps != "" {
		pageSize, _ = strconv.Atoi(ps)
	}

	users := []model.User{
		{ID: "1", Name: "张三", Email: "zhangsan@example.com", Age: 25},
		{ID: "2", Name: "李四", Email: "lisi@example.com", Age: 30},
	}

	c.JSON(http.StatusOK, model.UserListResponse{
		Success: true,
		Data: model.PaginatedUsers{
			Users:    users,
			Total:    len(users),
			Page:     page,
			PageSize: pageSize,
		},
		Message: "用户列表",
	})
}
