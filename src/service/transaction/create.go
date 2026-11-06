package transaction

import (
	"context"
	"ledger/src/models"
	"ledger/src/pkg/answer"
	"ledger/src/pkg/common"
	"ledger/src/service/instance"
	"ledger/src/slog"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/shopspring/decimal"
)

// Detail 账目明细
type Detail struct {
	Name     string          `json:"name"`     // 物品名称（如“牛奶”）
	Quantity decimal.Decimal `json:"quantity"` // 数量（如：2件商品、2次服务）
	Unit     string          `json:"unit"`     // 单位
	Price    decimal.Decimal `json:"price"`    // 单价
	Total    decimal.Decimal `json:"total"`    // 总价（数目 × 单价，与原 amount 绝对值匹配）
}

// ReqTransaction 请求数据
type ReqTransaction struct {
	Cid     int64           `json:"cid"`      // 类型ID
	OccTime string          `json:"occ_time"` // 发生的时间ISO 8601, 20220909T18:47:66+0800
	Amount  decimal.Decimal `json:"amount"`   // 金额 0.0000
	Remark  string          `json:"remark"`   // 备注
	Total   int8            `json:"total"`    // 是否计入本月收支，1：计入；0：不计入
	Detail  []*Detail       `json:"detail"`
}

// 转换时间字符串为毫秒时间戳
func timeToMillisecond(timeStr string) int64 {
	// https://pkg.go.dev/time#pkg-constants
	layout := "2006-01-02T15:04:05-0700"
	t, err := time.Parse(layout, timeStr)
	if err != nil {
		return 0
	}

	return t.UnixMilli()
}

func CreateTransaction() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		userId := c.GetString("userId")
		var ReqData ReqTransaction
		if err := c.BindJSON(&ReqData); err != nil {
			klog.Errorf("bind json failed %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to parse request data. Check JSON format.", ""))
			return
		}

		// 查询用户的实例ID
		instanceId, err := instance.GetInstanceId(userId)
		if err != nil {
			klog.Errorf("get instance id failed  %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to get your account info. Try again later.", ""))
			return
		}

		now := common.CreateTimestamp()
		dbData := &models.OrmTransaction{
			InstanceId: instanceId,
			CategoryId: ReqData.Cid,
			Amount:     ReqData.Amount,
			OccTime:    ReqData.OccTime,
			OccAt:      timeToMillisecond(ReqData.OccTime),
			Remark:     ReqData.Remark,
			UpdateTime: now,
			CreateTime: now,
		}

		if len(ReqData.Detail) <= 0 {
			// 没有明细
			err = models.InstallTransaction(dbData)
			if err != nil {
				klog.Errorf("insert transaction failed %v", err)
				c.JSON(http.StatusInternalServerError,
					answer.ResBody(answer.EcodeError,
						"Failed to create transaction. Try again later.", ""))
				return
			}
		} else {
			detail := make([]*models.OrmDetail, 0)
			// 有明细详情
			for _, val := range ReqData.Detail {
				detail = append(detail, &models.OrmDetail{
					Name:     val.Name,
					Quantity: val.Quantity,
					Price:    val.Price,
					Unit:     val.Unit,
					Total:    val.Total,
				})
			}
			err = models.InstallTransactionAndDetail(dbData, detail)
			if err != nil {
				klog.Errorf("insert transaction failed %v", err)
				c.JSON(http.StatusInternalServerError,
					answer.ResBody(answer.EcodeError,
						"Failed to create transaction. Try again later.", ""))
				return
			}
		}

		c.JSON(http.StatusCreated, answer.ResBody(answer.EcodeOK, nil, nil))
	}
}
