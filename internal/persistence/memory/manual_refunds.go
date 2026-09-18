package memory

import (
	"context"
	"sync"

	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
)

type ManualRefundRecordRepository struct {
	mutex   sync.RWMutex
	records map[domain.ID]disputes.ManualRefundRecord
}

func NewManualRefundRecordRepository() *ManualRefundRecordRepository {
	return &ManualRefundRecordRepository{records: make(map[domain.ID]disputes.ManualRefundRecord)}
}

func (repository *ManualRefundRecordRepository) SaveIfAbsent(_ context.Context, record disputes.ManualRefundRecord) (disputes.ManualRefundRecord, bool, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if stored, exists := repository.records[record.DisputeID]; exists {
		if stored != record {
			return disputes.ManualRefundRecord{}, false, disputes.ErrRemediationConflict
		}
		return stored, false, nil
	}
	repository.records[record.DisputeID] = record
	return record, true, nil
}
