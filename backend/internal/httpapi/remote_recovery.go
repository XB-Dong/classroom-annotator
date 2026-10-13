package httpapi

import (
	"context"
	"time"
)

// ReconcileRemote checks persisted operation identities after a cloud restart.
// It confirms committed outputs with the agent and never deletes unacked media.
func (s *SourceService) ReconcileRemote(ctx context.Context) error {
	var afterCreatedAt, afterID string
	for {
		operations, err := s.Store.PendingOperationsAfter(ctx, afterCreatedAt, afterID)
		if err != nil {
			return err
		}
		for _, ledger := range operations {
			afterCreatedAt, afterID = ledger.CreatedAt, ledger.OperationID
			if ctx.Err() != nil {
				return ctx.Err()
			}
			operationCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if s.Bridge == nil {
				if ledger.PublicationStatus == "unpublished" {
					if err := s.Store.MarkFailed(operationCtx, ledger.OperationID, "REMOTE_CLEANUP_PENDING"); err != nil {
						cancel()
						return err
					}
				}
				cancel()
				continue
			}
			if ledger.PublicationStatus == "committed" {
				remote, remoteErr := s.Bridge.Ack(operationCtx, ledger.OperationID, ledger.PublicationRevision)
				if remoteErr == nil && remote.PublicationStatus == "committed" {
					_ = s.Store.MarkAcknowledged(operationCtx, ledger.OperationID, ledger.PublicationRevision)
				}
				cancel()
				continue
			}
			remote, remoteErr := s.Bridge.Operation(operationCtx, ledger.OperationID)
			if remoteErr != nil {
				if err := s.Store.MarkFailed(operationCtx, ledger.OperationID, "REMOTE_CLEANUP_PENDING"); err != nil {
					cancel()
					return err
				}
				cancel()
				continue
			}
			if remote.Status == "queued" || remote.Status == "running" {
				remote, remoteErr = s.Bridge.Cancel(operationCtx, ledger.OperationID)
			}
			if remoteErr != nil || remote.Status != "cancelled" {
				if err := s.Store.MarkFailed(operationCtx, ledger.OperationID, "REMOTE_CLEANUP_PENDING"); err != nil {
					cancel()
					return err
				}
			} else {
				if err := s.Store.MarkFailed(operationCtx, ledger.OperationID, "INTERRUPTED"); err != nil {
					cancel()
					return err
				}
			}
			cancel()
		}
		if len(operations) < 1000 {
			return nil
		}
	}
}
