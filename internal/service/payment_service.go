package service

import (
	"cleanmark/internal/model"
	"cleanmark/internal/repository"
	"cleanmark/internal/utils"
	"cleanmark/pkg/errors"
	"time"

	"gorm.io/gorm"
)

type PaymentService struct {
	db *gorm.DB
}

func NewPaymentService() *PaymentService {
	return &PaymentService{
		db: repository.GetDB(),
	}
}

type Product struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       float64  `json:"price"`
	OriginalPrice float64 `json:"original_price"`
	Duration    string   `json:"duration"`
	Features    []string `json:"features"`
	Popular     bool     `json:"popular"`
}

type CreateOrderRequest struct {
	UserID      uint   `json:"user_id" binding:"required"`
	ProductType string `json:"product_type" binding:"required"` // monthly/yearly/lifetime/single
	PayMethod   string `json:"pay_method" binding:"required"`   // wechat/alipay
}

type OrderResponse struct {
	OrderNo     string    `json:"order_no"`
	Product     *Product  `json:"product"`
	PayURL      string    `json:"pay_url,omitempty"`
	QRCode      string    `json:"qr_code,omitempty"`
	Amount      float64   `json:"amount"`
	ExpireTime  string    `json:"expire_time"`
}

var Products = map[string]*Product{
	"monthly": {
		ID:          "pro_monthly",
		Name:        "Pro 月卡",
		Description: "适合轻度使用者",
		Price:       19.9,
		OriginalPrice: 39.9,
		Duration:    "30天",
		Features: []string{
			"每日无限次解析",
			"支持8大平台",
			"高清画质下载",
			"优先处理队列",
		},
	},
	"yearly": {
		ID:          "pro_yearly",
		Name:        "Pro 年卡",
		Description: "最受欢迎，性价比之选",
		Price:       199,
		OriginalPrice: 399,
		Duration:    "365天",
		Features: []string{
			"每日无限次解析",
			"支持8大平台",
			"超清画质下载",
			"优先处理队列",
			"批量处理(20个)",
			"API接口调用",
		},
		Popular: true,
	},
	"lifetime": {
		ID:          "pro_lifetime",
		Name:        "终身会员",
		Description: "一次购买，永久使用",
		Price:       599,
		OriginalPrice: 1199,
		Duration:    "永久",
		Features: []string{
			"所有月卡功能",
			"终身免费升级",
			"专属客服支持",
			"新功能优先体验",
			"API无限调用",
		},
	},
	"single": {
		ID:          "single_pack",
		Name:        "单次包",
		Description: "临时需要，按需购买",
		Price:       2.0,
		OriginalPrice: 5.0,
		Duration:    "1次",
		Features: []string{
			"单次解析额度",
			"支持所有平台",
			"高清画质下载",
			"24小时内有效",
		},
	},
}

func (s *PaymentService) GetProducts() []*Product {
	var productList []*Product
	for _, product := range Products {
		productList = append(productList, product)
	}
	return productList
}

func (s *PaymentService) CreateOrder(req *CreateOrderRequest) (*OrderResponse, error) {
	product, exists := Products[req.ProductType]
	if !exists {
		return nil, errors.New(400, "无效的产品类型")
	}

	orderNo := utils.GenerateOrderNo()
	expireTime := time.Now().Add(30 * time.Minute)

	order := &model.Order{
		UserID:      req.UserID,
		OrderNo:     orderNo,
		ProductType: req.ProductType,
		Amount:      product.Price,
		PayMethod:   req.PayMethod,
		Status:      "pending",
		ExpireTime:  &expireTime,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.db.Create(order).Error; err != nil {
		return nil, errors.ErrInternalServer
	}

	response := &OrderResponse{
		OrderNo:    orderNo,
		Product:    product,
		Amount:     product.Price,
		ExpireTime: expireTime.Format("2006-01-02 15:04:05"),
	}

	if req.PayMethod == "wechat" {
		payData, err := s.createWechatPay(orderNo, product.Price)
		if err != nil {
			return nil, errors.New(500, "创建微信支付失败")
		}
		response.PayURL = payData["code_url"]
		response.QRCode = payData["code_url"]
	} else if req.PayMethod == "alipay" {
		payURL, err := s.createAlipay(orderNo, product.Price)
		if err != nil {
			return nil, errors.New(500, "创建支付宝支付失败")
		}
		response.PayURL = payURL
		response.QRCode = payURL
	}

	return response, nil
}

func (s *PaymentService) createWechatPay(orderNo string, amount float64) (map[string]string, error) {
	result := make(map[string]string)
	
	result["code_url"] = "weixin://wxpay/bizpayurl?pr=" + orderNo
	
	return result, nil
}

func (s *PaymentService) createAlipay(orderNo string, amount float64) (string, error) {
	payURL := "https://openapi.alipay.com/gateway.do?" + orderNo
	
	return payURL, nil
}

func (s *PaymentService) HandlePaymentCallback(orderNo string, transactionID string) error {
	order := &model.Order{}
	if err := s.db.Where("order_no = ?", orderNo).First(order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.ErrNotFound
		}
		return errors.ErrInternalServer
	}

	if order.Status == "paid" {
		return nil
	}

	paidTime := time.Now()

	s.db.Model(order).Updates(map[string]interface{}{
		"status":   "paid",
		"pay_time": paidTime,
	})

	s.activateVip(order.UserID, order.ProductType)

	return nil
}

func (s *PaymentService) activateVip(userID uint, productType string) {
	user := &model.User{}
	if err := s.db.First(user, userID).Error; err != nil {
		return
	}

	var vipLevel int
	var vipExpireTime *time.Time

	switch productType {
	case "monthly":
		vipLevel = 1
		expire := time.Now().AddDate(0, 1, 0)
		vipExpireTime = &expire
	case "yearly":
		vipLevel = 2
		expire := time.Now().AddDate(1, 0, 0)
		vipExpireTime = &expire
	case "lifetime":
		vipLevel = 3
		future := time.Date(2099, 12, 31, 23, 59, 59, 0, time.Local)
		vipExpireTime = &future
	case "single":
		currentQuota := user.DailyQuota
		s.db.Model(user).Update("daily_quota", currentQuota+1)
		return
	default:
		return
	}

	updates := map[string]interface{}{
		"vip_level":      vipLevel,
		"vip_expire_time": vipExpireTime,
		"daily_quota":     999999,
	}

	s.db.Model(user).Updates(updates)
}

func (s *PaymentService) GetOrders(userID uint, page, pageSize int) ([]*model.Order, int64, error) {
	var orders []model.Order
	var total int64

	query := s.db.Where("user_id = ?", userID)

	query.Model(&model.Order{}).Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&orders).Error

	if err != nil {
		return nil, 0, errors.ErrInternalServer
	}

	result := make([]*model.Order, len(orders))
	for i := range orders {
		result[i] = &orders[i]
	}

	return result, total, nil
}
