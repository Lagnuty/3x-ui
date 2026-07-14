package job

import (
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

type OutboundSubscriptionJob struct {
	subService *service.OutboundSubscriptionService
	xraySvc    *service.XrayService
}

func NewOutboundSubscriptionJob() *OutboundSubscriptionJob {
	return &OutboundSubscriptionJob{
		subService: &service.OutboundSubscriptionService{},
		xraySvc:    &service.XrayService{},
	}
}

func (j *OutboundSubscriptionJob) Run() {
	if j.subService == nil {
		j.subService = &service.OutboundSubscriptionService{}
	}
	if j.xraySvc == nil {
		j.xraySvc = &service.XrayService{}
	}
	count, err := j.subService.RefreshAllEnabled()
	if err != nil {
		logger.Warning("outbound subscription auto-update error:", err)
		return
	}
	if count > 0 {
		logger.Infof("Refreshed %d outbound subscription(s)", count)
		j.xraySvc.SetToNeedRestart()
	}
}
