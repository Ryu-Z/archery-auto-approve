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
	autoApprove := utils.IsAutoApproveTime(now, s.cfg.Schedule)
	s.logger.Info("polling workflows",
		utils.FieldTime("now", now),
		utils.FieldBool("auto_approve", autoApprove),
		utils.FieldString("timezone", s.cfg.Schedule.Timezone),
		utils.FieldStrings("workdays", s.cfg.Schedule.Workdays),
		utils.FieldString("business_hours_start", s.cfg.Schedule.BusinessHours.Start),
		utils.FieldString("business_hours_end", s.cfg.Schedule.BusinessHours.End),
		utils.FieldBool("weekends_auto_approve", s.cfg.Schedule.WeekendsAutoApprove),
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
				utils.FieldInt("id", workflow.PrimaryID()),
				utils.FieldInt("workflow_id", workflow.EffectiveID()),
				utils.FieldInt("workflow_type", workflow.EffectiveWorkflowType()),
				utils.FieldString("name", workflow.DisplayName()),
				utils.FieldString("db_name", workflow.EffectiveDBName()),
				utils.FieldString("create_time", workflow.EffectiveCreateTime()),
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
					utils.FieldInt("id", wf.PrimaryID()),
					utils.FieldInt("workflow_id", wf.EffectiveID()),
					utils.FieldInt("workflow_type", wf.EffectiveWorkflowType()),
					utils.FieldString("name", wf.DisplayName()),
					utils.FieldString("db_name", wf.EffectiveDBName()),
					utils.FieldString("create_time", wf.EffectiveCreateTime()),
					utils.FieldError(err),
				)
				return
			}

			s.logger.Info("workflow approved",
				utils.FieldInt("id", wf.PrimaryID()),
				utils.FieldInt("workflow_id", wf.EffectiveID()),
				utils.FieldInt("workflow_type", wf.EffectiveWorkflowType()),
				utils.FieldString("name", wf.DisplayName()),
				utils.FieldString("db_name", wf.EffectiveDBName()),
				utils.FieldString("create_time", wf.EffectiveCreateTime()),
				utils.FieldString("status", wf.EffectiveStatus()),
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
