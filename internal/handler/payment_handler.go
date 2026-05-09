package handler

import (
	"cleanmark/internal/model"
	"cleanmark/internal/repository"
	"cleanmark/internal/service"
	"cleanmark/pkg/errors"
	"cleanmark/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	paymentService *service.PaymentService
}

func NewPaymentHandler() *PaymentHandler {
	return &PaymentHandler{
		paymentService: service.NewPaymentService(),
	}
}

func (h *PaymentHandler) GetProducts(c *gin.Context) {
	products := h.paymentService.GetProducts()
	
	response.Success(c, gin.H{
		"products": products,
	})
}

func (h *PaymentHandler) CreateOrder(c *gin.Context) {
	var req service.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	req.UserID = userID.(uint)

	result, err := h.paymentService.CreateOrder(&req)
	if err != nil {
		appErr := err.(*errors.AppError)
		response.Error(c, appErr.Code/100, appErr.Code, appErr.Message)
		return
	}

	response.SuccessWithMessage(c, "订单创建成功", result)
}

func (h *PaymentHandler) HandleWechatCallback(c *gin.Context) {
	orderNo := c.PostForm("out_trade_no")
	transactionID := c.PostForm("transaction_id")

	if orderNo == "" || transactionID == "" {
		response.BadRequest(c, "缺少必要参数")
		return
	}

	err := h.paymentService.HandlePaymentCallback(orderNo, transactionID)
	if err != nil {
		response.InternalError(c, "处理回调失败")
		return
	}

	c.String(200, "SUCCESS")
}

func (h *PaymentHandler) HandleAlipayCallback(c *gin.Context) {
	orderNo := c.PostForm("out_trade_no")
	tradeNo := c.PostForm("trade_no")

	if orderNo == "" || tradeNo == "" {
		response.BadRequest(c, "缺少必要参数")
		return
	}

	err := h.paymentService.HandlePaymentCallback(orderNo, tradeNo)
	if err != nil {
		response.InternalError(c, "处理回调失败")
		return
	}

	c.String(200, "success")
}

func (h *PaymentHandler) GetOrderList(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 50 { pageSize = 20 }

	list, total, err := h.paymentService.GetOrders(uid, page, pageSize)
	if err != nil {
		response.InternalError(c, "获取订单列表失败")
		return
	}

	response.PageSuccess(c, list, total, page, pageSize)
}

func (h *PaymentHandler) CheckOrderStatus(c *gin.Context) {
	orderNo := c.Query("order_no")

	if orderNo == "" {
		response.BadRequest(c, "请提供订单号")
		return
	}

	db := repository.GetDB()
	var order model.Order

	if err := db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		response.NotFound(c, "订单不存在")
		return
	}

	response.Success(c, gin.H{
		"order_no": order.OrderNo,
		"status":   order.Status,
		"paid_at":  order.PayTime,
	})
}
