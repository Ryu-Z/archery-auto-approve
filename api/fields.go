package api

import "go.uber.org/zap"

func fieldWorkflowID(id int) zap.Field {
	return zap.Int("workflow_id", id)
}

func fieldStatus(status string) zap.Field {
	return zap.String("status", status)
}

func fieldWorkflowType(workflowType int) zap.Field {
	return zap.Int("workflow_type", workflowType)
}

func fieldPath(path string) zap.Field {
	return zap.String("path", path)
}

func fieldAttempt(attempt int) zap.Field {
	return zap.Int("attempt", attempt)
}

func fieldRetryMax(max int) zap.Field {
	return zap.Int("retry_max", max)
}

func fieldError(err error) zap.Field {
	return zap.Error(err)
}
