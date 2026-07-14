package job

import (
	"context"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

type NodeHeartbeatJob struct {
	nodeService *service.NodeService
	mu          sync.Mutex
	running     bool
}

func NewNodeHeartbeatJob() *NodeHeartbeatJob {
	return &NodeHeartbeatJob{nodeService: &service.NodeService{}}
}

func (j *NodeHeartbeatJob) Run() {
	j.mu.Lock()
	if j.running {
		j.mu.Unlock()
		return
	}
	j.running = true
	j.mu.Unlock()
	defer func() {
		j.mu.Lock()
		j.running = false
		j.mu.Unlock()
	}()

	if j.nodeService == nil {
		j.nodeService = &service.NodeService{}
	}
	nodes, err := j.nodeService.GetAll()
	if err != nil {
		logger.Warning("node heartbeat list error:", err)
		return
	}
	for _, node := range nodes {
		if node == nil || !node.Enable {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		patch, probeErr := j.nodeService.Probe(ctx, node)
		cancel()
		if probeErr != nil {
			patch.Status = "offline"
		} else {
			patch.Status = "online"
		}
		if err := j.nodeService.UpdateHeartbeat(node.Id, patch); err != nil {
			logger.Warningf("node heartbeat update error for node %d: %v", node.Id, err)
		}
	}
}
