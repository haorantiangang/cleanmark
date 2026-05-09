package tests

import (
	"cleanmark/internal/adapter"
	"cleanmark/internal/service"
	"testing"
)

func TestPaymentService_GetProducts(t *testing.T) {
	svc := service.NewPaymentService()

	products := svc.GetProducts()

	if len(products) != 4 {
		t.Errorf("应该有4个产品，得到: %d", len(products))
	}

	productMap := make(map[string]*service.Product)
	for _, p := range products {
		productMap[p.ID] = p
	}

	expectedProducts := []string{"pro_monthly", "pro_yearly", "pro_lifetime", "single_pack"}
	for _, expectedID := range expectedProducts {
		if _, exists := productMap[expectedID]; !exists {
			t.Errorf("缺少产品: %s", expectedID)
		}
	}
}

func TestPaymentService_ProductDetails(t *testing.T) {
	svc := service.NewPaymentService()
	products := svc.GetProducts()

	monthly := products[0]
	
	if monthly.Name != "Pro 月卡" {
		t.Errorf("月卡名称不正确，得到: %s", monthly.Name)
	}

	if monthly.Price != 19.9 {
		t.Errorf("月卡价格不正确，得到: %.2f", monthly.Price)
	}

	if monthly.Duration != "30天" {
		t.Errorf("月卡时长不正确，得到: %s", monthly.Duration)
	}

	if len(monthly.Features) == 0 {
		t.Error("月卡功能列表不应该为空")
	}
}

func TestPaymentService_YearlyIsPopular(t *testing.T) {
	svc := service.NewPaymentService()
	products := svc.GetProducts()

	var yearly *service.Product
	for _, p := range products {
		if p.ID == "pro_yearly" {
			yearly = p
			break
		}
	}

	if yearly == nil {
		t.Fatal("年卡产品不存在")
	}

	if !yearly.Popular {
		t.Error("年卡应该是热门推荐产品")
	}
}

func TestPaymentService_LifetimePrice(t *testing.T) {
	svc := service.NewPaymentService()
	products := svc.GetProducts()

	var lifetime *service.Product
	for _, p := range products {
		if p.ID == "pro_lifetime" {
			lifetime = p
			break
		}
	}

	if lifetime == nil {
		t.Fatal("终身会员产品不存在")
	}

	if lifetime.Price != 599 {
		t.Errorf("终身会员价格不正确，得到: %.2f", lifetime.Price)
	}

	if lifetime.Duration != "永久" {
		t.Errorf("终身会员时长应该是'永久'，得到: %s", lifetime.Duration)
	}
}

func TestPaymentService_SinglePack(t *testing.T) {
	svc := service.NewPaymentService()
	products := svc.GetProducts()

	var single *service.Product
	for _, p := range products {
		if p.ID == "single_pack" {
			single = p
			break
		}
	}

	if single == nil {
		t.Fatal("单次包产品不存在")
	}

	if single.Price != 2.0 {
		t.Errorf("单次包价格不正确，得到: %.2f", single.Price)
	}

	if single.Duration != "1次" {
		t.Errorf("单次包时长应该是'1次'，得到: %s", single.Duration)
	}
}

func TestPaymentService_CreateOrderValidation(t *testing.T) {
	svc := service.NewPaymentService()

	t.Run("无效的产品类型", func(t *testing.T) {
		req := &service.CreateOrderRequest{
			UserID:      1,
			ProductType: "invalid_product",
			PayMethod:   "wechat",
		}

		_, err := svc.CreateOrder(req)
		if err == nil {
			t.Error("无效产品类型应该返回错误")
		}
	})

	t.Run("缺少必要参数", func(t *testing.T) {
		req := &service.CreateOrderRequest{
			UserID:      1,
			ProductType: "",
			PayMethod:   "",
		}

		_, err := svc.CreateOrder(req)
		if err == nil {
			t.Error("缺少参数应该返回错误")
		}
	})
}

func TestPlatformAdapterRegistry(t *testing.T) {
	supportedPlatforms := []string{
		"douyin",
		"kuaishou",
		"xiaohongshu",
		"bilibili",
		"weibo",
		"xigua",
		"youtube",
		"tiktok",
	}

	for _, platform := range supportedPlatforms {
		t.Run(platform, func(t *testing.T) {
			adp := adapter.GetAdapter(platform)
			if adp == nil {
				t.Errorf("%s 平台的适配器不应该为nil", platform)
			}

			if adp.Name() == "" {
				t.Errorf("%s 适配器的名称不应该为空", platform)
			}

			domains := adp.SupportedDomains()
			if len(domains) == 0 {
				t.Errorf("%s 适配器的支持域名列表不应该为空", platform)
			}
		})
	}

	t.Run("不支持的平台返回nil", func(t *testing.T) {
		adp := adapter.GetAdapter("unsupported_platform")
		if adp != nil {
			t.Error("不支持的平台应该返回nil适配器")
		}
	})
}

func TestGetAllAdapters(t *testing.T) {
	allAdapters := adapter.GetAllAdapters()

	if len(allAdapters) != 8 {
		t.Errorf("应该有8个适配器，得到: %d", len(allAdapters))
	}

	platformNames := make(map[string]bool)
	for _, adp := range allAdapters {
		if platformNames[adp.Platform] {
			t.Errorf("重复的平台: %s", adp.Platform)
		}
		platformNames[adp.Platform] = true
		
		if adp.Name == "" {
			t.Errorf("平台 %s 的名称为空", adp.Platform)
		}
		
		if len(adp.Domains) == 0 {
			t.Errorf("平台 %s 的域名为空", adp.Platform)
		}
	}
}
