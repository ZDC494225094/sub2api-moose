package service

import (
	"context"
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
			if r.cancel != nil && !isTerminalPlaygroundRunStatus(r.Status) {
				r.cancel()
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
					if r != nil && r.cancel != nil {
						r.cancel()
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
		success := runErr == nil && ctx.Err() == nil
		charge, err := s.canvasBilling.finish(request.canvasCharge.ID, success)
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
