package service

import (
	"context"
	"errors"
	"log"
	"time"
)

func (s *PlaygroundRunService) Stop() {
	if s == nil {
		return
	}
	s.stopBillingOnce.Do(func() {
		if s.stopBilling != nil {
			close(s.stopBilling)
		}
		s.mu.RLock()
		defer s.mu.RUnlock()
		for _, r := range s.runs {
			if r.cancelCause != nil && !isTerminalPlaygroundRunStatus(r.Status) {
				r.cancelCause(errors.New("服务正在关闭，任务执行已中断"))
			}
		}
	})
}
func (s *PlaygroundRunService) recoverCanvasCharges() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		s.canvasBilling.recoverExpired(ctx)
		cancel()
		select {
		case <-s.stopBilling:
			return
		case <-ticker.C:
		}
	}
}
func (s *PlaygroundRunService) heartbeatCanvasCharge(ctx context.Context, key string, request PlaygroundRunRequest) func() {
	if request.canvasCharge == nil {
		return func() {}
	}
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				renewal, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				live, err := s.canvasBilling.repo.RenewCanvasCharge(renewal, request.canvasCharge.ID)
				cancel()
				if err != nil {
					log.Printf("canvas billing heartbeat failed for %s: %v", request.ID, err)
				}
				if err != nil || !live {
					// Stop the producer before an expired lease can be refunded.
					s.mu.RLock()
					r := s.runs[key]
					if r != nil && r.cancelCause != nil {
						reason := "任务执行租约已失效，已停止生成"
						if err != nil {
							reason = "任务计费心跳续期失败：" + sanitizeCanvasRunError(err.Error())
						}
						r.cancelCause(errors.New(reason))
					}
					s.mu.RUnlock()
					return
				}
			}
		}
	}()
	return func() { close(stop) }
}

func (s *PlaygroundRunService) settleCanvasCharge(ctx context.Context, request PlaygroundRunRequest, runErr error) (*CanvasCharge, error) {
	for {
		s.mu.RLock()
		run := s.runs[playgroundRunKey(request.canvasCharge.UserID, request.ID)]
		diagnostic := CanvasRunDiagnostics{Phase: "queued"}
		if run != nil {
			diagnostic.Phase, diagnostic.UpstreamTaskID = run.Phase, run.UpstreamTaskID
		}
		s.mu.RUnlock()
		if runErr != nil {
			diagnostic.Error = runErr.Error()
		} else if ctx.Err() != nil {
			diagnostic.Error = context.Cause(ctx).Error()
		} else {
			diagnostic.Phase = "completed"
		}
		success := runErr == nil && ctx.Err() == nil
		// Commit the cause and refund together, including queries on other instances.
		charge, err := s.canvasBilling.finishWithDiagnostics(request.canvasCharge.ID, success, diagnostic)
		if err == nil {
			return charge, nil
		}
		log.Printf("canvas billing settlement pending for run %s: %v", request.ID, err)
		// Do not publish success (or a refunded failure) until the money
		// transaction commits. An interrupted process is covered by lease recovery.
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-s.stopBilling:
			timer.Stop()
			return nil, err
		case <-timer.C:
		}
	}
}
