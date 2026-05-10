package api

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"token_blockchain/database"
	"token_blockchain/middleware"
	"token_blockchain/service"
	"token_blockchain/utils"
)

type Server struct {
	novelService *service.NovelService
	userService  *service.UserService
}

func NewServer() *Server {
	if err := utils.InitRSACrypto(); err != nil {
		log.Printf("警告: RSA加密解密器初始化失败: %v", err)
	}

	return &Server{
		novelService: service.NewNovelService(),
		userService:  service.NewUserService(),
	}
}

func (s *Server) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", s.healthCheck)

	api := r.Group("/api/v1")
	{
		novels := api.Group("/novels")
		{
			novels.GET("", s.getAllNovels)
			novels.GET("/:id", s.getNovel)
			novels.DELETE("/:id", s.deleteNovel)

			encryptedNovels := novels.Group("")
			encryptedNovels.Use(middleware.RSARequestMiddleware())
			{
				encryptedNovels.POST("", s.createNovel)
				encryptedNovels.PUT("/:id", s.updateNovel)
			}
		}

		users := api.Group("/users")
		{
			users.GET("", s.getAllUserCredits)
			users.GET("/:id", s.getUserCredit)
			users.DELETE("/:id", s.deleteUserCredit)
			users.POST("/recharge", s.rechargeUserTokens)

			encryptedUsers := users.Group("")
			encryptedUsers.Use(middleware.RSARequestMiddleware())
			{
				encryptedUsers.POST("", s.createUserCredit)
				encryptedUsers.PUT("/:id", s.updateUserCredit)
				encryptedUsers.POST("/:id/consume-token", s.consumeUserToken)
			}
		}
	}
}

func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) getAllNovels(c *gin.Context) {
	novels, err := s.novelService.GetAllNovels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, novels)
}

func (s *Server) getNovel(c *gin.Context) {
	id := c.Param("id")
	novel, err := s.novelService.GetNovel(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, novel)
}

func (s *Server) createNovel(c *gin.Context) {
	var novel database.Novel
	if err := c.ShouldBindJSON(&novel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.novelService.CreateNovel(&novel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, novel)
}

func (s *Server) updateNovel(c *gin.Context) {
	id := c.Param("id")
	var novel database.Novel
	if err := c.ShouldBindJSON(&novel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.novelService.UpdateNovel(id, &novel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, novel)
}

func (s *Server) deleteNovel(c *gin.Context) {
	id := c.Param("id")
	if err := s.novelService.DeleteNovel(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (s *Server) getAllUserCredits(c *gin.Context) {
	userCredits, err := s.userService.GetAllUserCredits()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, userCredits)
}

func (s *Server) getUserCredit(c *gin.Context) {
	userId := c.Param("id")
	uc, err := s.userService.GetUserCredit(userId)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, uc)
}

func (s *Server) createUserCredit(c *gin.Context) {
	var uc database.UserCredit
	if err := c.ShouldBindJSON(&uc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.userService.CreateUserCredit(&uc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, uc)
}

func (s *Server) updateUserCredit(c *gin.Context) {
	userId := c.Param("id")
	var uc database.UserCredit
	if err := c.ShouldBindJSON(&uc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.userService.UpdateUserCredit(userId, &uc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, uc)
}

func (s *Server) deleteUserCredit(c *gin.Context) {
	userId := c.Param("id")
	if err := s.userService.DeleteUserCredit(userId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

type RechargeCallbackRequest struct {
	Title       string `json:"title"`
	OrderSN     string `json:"order_sn" binding:"required"`
	Email       string `json:"email" binding:"required"`
	ActualPrice int    `json:"actual_price" binding:"required"`
	OrderInfo   string `json:"order_info"`
	GoodID      string `json:"good_id"`
	GoodName    string `json:"gd_name"`
	Timestamp   string `json:"timestamp" binding:"required"`
	Signature   string `json:"signature" binding:"required"`
}

func (s *Server) rechargeUserTokens(c *gin.Context) {
	var req RechargeCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	timestampInt, err := service.ParseTimestamp(req.Timestamp)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid timestamp format"})
		return
	}

	if err := service.ValidateTimestamp(timestampInt); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "timestamp validation failed: " + err.Error()})
		return
	}

	params := map[string]string{
		"actual_price": strconv.Itoa(req.ActualPrice),
		"email":        req.Email,
		"order_sn":     req.OrderSN,
		"timestamp":    req.Timestamp,
	}

	if !service.ValidateHMACSignature(params, req.Signature, service.GetRechargeSecretKey()) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "signature verification failed"})
		return
	}

	uc, err := s.userService.Recharge(req.Email, req.ActualPrice, req.OrderInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, uc)
}

type ConsumeRequest struct {
	Amount      int    `json:"amount"`
	NovelID     string `json:"novelId"`
	Description string `json:"description"`
}

func (s *Server) consumeUserToken(c *gin.Context) {
	userId := c.Param("id")
	var req ConsumeRequest
	req.Amount = 1

	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("[consumeUserToken] bind error: %v, userId: %s, body: %v", err, userId, c.Request.Body)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Amount <= 0 {
			req.Amount = 1
		}
	}

	uc, err := s.userService.Consume(userId, req.Amount, req.NovelID, req.Description)
	if err != nil {
		if errors.Is(err, service.ErrInsufficientCredit) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient credit"})
			return
		}
		if errors.Is(err, service.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, uc)
}
