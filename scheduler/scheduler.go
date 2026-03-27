package scheduler

import (
	"context"
	"sync"
	"time"

	"archery-auto-approve/api"
	"archery-auto-approve/config"
	"archery-auto-approve/model"
	"archery-auto-approve/utils"
)

type Scheduler struct {
	cfg    *config.Config
	client *api.Client
	logger utils.Logger
}

func New(cfg *config.Config, client *api.Client, logger utils.Logger) *Scheduler {
	return &Scheduler{
		cfg:    cfg,
		client: client,
		logger: logger,
	}
}

func (s *Scheduler) Run(ctx context.Context) error {
	s.logger.Info("scheduler started")

	if err := s.runOnce(ctx); err != nil {
		s.logger.Error("initial polling failed", utils.FieldError(err))
	}

	ticker := timeTicker(s.cfg.PollDuration())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.runOnce(ctx); err != nil && ctx.Err() == nil {
				s.logger.Error("polling cycle failed", utils.FieldError(err))
			}
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context) error {
	now := utils.BeijingTime()
	autoApprove := utils.IsAutoApproveTime(now)
	s.logger.Info("polling workflows",
		utils.FieldTime("now", now),
		utils.FieldBool("auto_approve", autoApprove),
	)

	if !autoApprove {
		return nil
	}

	workflows, err := s.client.ListPendingWorkflows(ctx, s.cfg.PendingStatuses)
	if err != nil {
		return err
	}
	if len(workflows) == 0 {
		s.logger.Info("no pending workflows found")
		return nil
	}

	s.logger.Info("pending workflows loaded", utils.FieldInt("count", len(workflows)))

	sem := make(chan struct{}, s.cfg.MaxConcurrent)
	var wg sync.WaitGroup
	approver := s.cfg.Approver
	if approver == "" {
		approver = s.cfg.Archery.Username
	}

	for _, workflow := range workflows {
		workflow := workflow
		if workflow.IsApprovedLike() {
			s.logger.Info("skip already approved workflow",
				utils.FieldInt("workflow_id", workflow.EffectiveID()),
				utils.FieldInt("workflow_type", workflow.EffectiveWorkflowType()),
				utils.FieldString("status", workflow.EffectiveStatus()),
			)
			continue
		}

		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(wf model.Workflow) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := s.client.ApproveWorkflow(ctx, wf, s.cfg.ApprovalRemark, approver); err != nil {
				s.logger.Error("auto approve failed",
					utils.FieldInt("workflow_id", wf.EffectiveID()),
					utils.FieldInt("workflow_type", wf.EffectiveWorkflowType()),
					utils.FieldError(err),
				)
				return
			}

			s.logger.Info("workflow approved",
				utils.FieldInt("workflow_id", wf.EffectiveID()),
				utils.FieldInt("workflow_type", wf.EffectiveWorkflowType()),
				utils.FieldString("name", wf.DisplayName()),
				utils.FieldString("remark", s.cfg.ApprovalRemark),
			)
		}(workflow)
	}

	wg.Wait()
	return nil
}

func timeTicker(d time.Duration) *time.Ticker {
	return time.NewTicker(d)
}
