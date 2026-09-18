package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/fourgeez/agentpay/internal/approvals"
	"github.com/fourgeez/agentpay/internal/catalog"
	"github.com/fourgeez/agentpay/internal/disputes"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/intents"
	"github.com/fourgeez/agentpay/internal/persistence"
	"github.com/fourgeez/agentpay/internal/transactions"
)

var _ domain.IdempotencyStore = (*IdempotencyStore)(nil)

type CatalogRepository struct {
	mutex                sync.RWMutex
	sellers              map[domain.ID]catalog.Seller
	sellerBySlug         map[string]domain.ID
	sellerByOwnerSubject map[string]domain.ID
	routes               map[domain.ID]catalog.PaidRoute
	routesBySeller       map[domain.ID][]domain.ID
	productSlugsBySeller map[domain.ID]map[string]domain.ID
}

func NewCatalogRepository() *CatalogRepository {
	return &CatalogRepository{
		sellers:              make(map[domain.ID]catalog.Seller),
		sellerBySlug:         make(map[string]domain.ID),
		sellerByOwnerSubject: make(map[string]domain.ID),
		routes:               make(map[domain.ID]catalog.PaidRoute),
		routesBySeller:       make(map[domain.ID][]domain.ID),
		productSlugsBySeller: make(map[domain.ID]map[string]domain.ID),
	}
}

func (repository *CatalogRepository) CreateSeller(_ context.Context, seller catalog.Seller) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if _, exists := repository.sellers[seller.SellerID]; exists {
		return persistence.ErrAlreadyExists
	}
	if _, exists := repository.sellerBySlug[seller.Slug]; exists {
		return persistence.ErrAlreadyExists
	}
	if _, exists := repository.sellerByOwnerSubject[seller.OwnerSubject]; exists {
		return persistence.ErrAlreadyExists
	}
	repository.sellers[seller.SellerID] = seller
	repository.sellerBySlug[seller.Slug] = seller.SellerID
	repository.sellerByOwnerSubject[seller.OwnerSubject] = seller.SellerID
	return nil
}

func (repository *CatalogRepository) ResolveSellerByOwnerSubject(_ context.Context, ownerSubject string) (catalog.Seller, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	sellerID, exists := repository.sellerByOwnerSubject[ownerSubject]
	if !exists {
		return catalog.Seller{}, persistence.ErrNotFound
	}
	return repository.sellers[sellerID], nil
}

func (repository *CatalogRepository) GetSeller(_ context.Context, sellerID domain.ID) (catalog.Seller, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	seller, exists := repository.sellers[sellerID]
	if !exists {
		return catalog.Seller{}, persistence.ErrNotFound
	}
	return seller, nil
}

func (repository *CatalogRepository) ResolveSellerBySlug(_ context.Context, slug string) (catalog.Seller, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	sellerID, exists := repository.sellerBySlug[slug]
	if !exists {
		return catalog.Seller{}, persistence.ErrNotFound
	}
	return repository.sellers[sellerID], nil
}

func (repository *CatalogRepository) UpdateSeller(_ context.Context, seller catalog.Seller, expectedVersion uint64) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	stored, exists := repository.sellers[seller.SellerID]
	if !exists {
		return persistence.ErrNotFound
	}
	if stored.Version != expectedVersion || seller.Version != expectedVersion+1 || stored.Slug != seller.Slug {
		return persistence.ErrConditionFailed
	}
	repository.sellers[seller.SellerID] = seller
	return nil
}

func (repository *CatalogRepository) CreateRoute(_ context.Context, route catalog.PaidRoute) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if _, exists := repository.routes[route.RouteID]; exists {
		return persistence.ErrAlreadyExists
	}
	if _, exists := repository.sellers[route.SellerID]; !exists {
		return persistence.ErrNotFound
	}
	productSlugs := repository.productSlugsBySeller[route.SellerID]
	if productSlugs == nil {
		productSlugs = make(map[string]domain.ID)
		repository.productSlugsBySeller[route.SellerID] = productSlugs
	}
	if _, exists := productSlugs[route.ProductSlug]; exists {
		return persistence.ErrAlreadyExists
	}
	repository.routes[route.RouteID] = cloneRoute(route)
	repository.routesBySeller[route.SellerID] = append(repository.routesBySeller[route.SellerID], route.RouteID)
	productSlugs[route.ProductSlug] = route.RouteID
	return nil
}

func (repository *CatalogRepository) GetRoute(_ context.Context, routeID domain.ID) (catalog.PaidRoute, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	route, exists := repository.routes[routeID]
	if !exists {
		return catalog.PaidRoute{}, persistence.ErrNotFound
	}
	return cloneRoute(route), nil
}

func (repository *CatalogRepository) UpdateRoute(_ context.Context, route catalog.PaidRoute, expectedVersion uint64) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	stored, exists := repository.routes[route.RouteID]
	if !exists {
		return persistence.ErrNotFound
	}
	if stored.Version != expectedVersion || route.Version != expectedVersion+1 ||
		stored.SellerID != route.SellerID || stored.PathPattern != route.PathPattern ||
		(stored.ProductSlug != "" && stored.ProductSlug != route.ProductSlug) ||
		(stored.DisplayName != "" && stored.DisplayName != route.DisplayName) {
		return persistence.ErrConditionFailed
	}
	productSlugs := repository.productSlugsBySeller[route.SellerID]
	if productSlugs == nil {
		productSlugs = make(map[string]domain.ID)
		repository.productSlugsBySeller[route.SellerID] = productSlugs
	}
	if claimedRouteID, exists := productSlugs[route.ProductSlug]; exists && claimedRouteID != route.RouteID {
		return persistence.ErrConditionFailed
	}
	productSlugs[route.ProductSlug] = route.RouteID
	repository.routes[route.RouteID] = cloneRoute(route)
	return nil
}

func (repository *CatalogRepository) ListRoutesBySeller(_ context.Context, sellerID domain.ID) ([]catalog.PaidRoute, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	routeIDs := repository.routesBySeller[sellerID]
	routes := make([]catalog.PaidRoute, 0, len(routeIDs))
	for _, routeID := range routeIDs {
		routes = append(routes, cloneRoute(repository.routes[routeID]))
	}
	return routes, nil
}

type PurchaseIntentRepository struct {
	mutex   sync.RWMutex
	intents map[domain.ID]intents.PurchaseIntent
}

func NewPurchaseIntentRepository() *PurchaseIntentRepository {
	return &PurchaseIntentRepository{intents: make(map[domain.ID]intents.PurchaseIntent)}
}

func (repository *PurchaseIntentRepository) Create(_ context.Context, purchaseIntent intents.PurchaseIntent) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if _, exists := repository.intents[purchaseIntent.IntentID()]; exists {
		return persistence.ErrAlreadyExists
	}
	repository.intents[purchaseIntent.IntentID()] = purchaseIntent
	return nil
}

func (repository *PurchaseIntentRepository) Get(_ context.Context, intentID domain.ID) (intents.PurchaseIntent, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	purchaseIntent, exists := repository.intents[intentID]
	if !exists {
		return intents.PurchaseIntent{}, persistence.ErrNotFound
	}
	return purchaseIntent, nil
}

type ApprovalRepository struct {
	mutex    sync.RWMutex
	sessions map[domain.ID]approvals.Session
}

func NewApprovalRepository() *ApprovalRepository {
	return &ApprovalRepository{sessions: make(map[domain.ID]approvals.Session)}
}

func (repository *ApprovalRepository) Create(_ context.Context, session approvals.Session) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if _, exists := repository.sessions[session.SessionID()]; exists {
		return persistence.ErrAlreadyExists
	}
	repository.sessions[session.SessionID()] = session.Clone()
	return nil
}

func (repository *ApprovalRepository) Get(_ context.Context, sessionID domain.ID) (approvals.Session, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	session, exists := repository.sessions[sessionID]
	if !exists {
		return approvals.Session{}, persistence.ErrNotFound
	}
	return session.Clone(), nil
}

func (repository *ApprovalRepository) Update(_ context.Context, session approvals.Session, expectedVersion uint64) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	stored, exists := repository.sessions[session.SessionID()]
	if !exists {
		return persistence.ErrNotFound
	}
	if stored.Version() != expectedVersion || session.Version() != expectedVersion+1 {
		return persistence.ErrConditionFailed
	}
	repository.sessions[session.SessionID()] = session.Clone()
	return nil
}

type TransactionRepository struct {
	mutex              sync.RWMutex
	transactions       map[domain.ID]transactions.Transaction
	paymentIdentifiers map[string]domain.ID
}

func NewTransactionRepository() *TransactionRepository {
	return &TransactionRepository{transactions: make(map[domain.ID]transactions.Transaction), paymentIdentifiers: make(map[string]domain.ID)}
}

func (repository *TransactionRepository) Create(_ context.Context, transaction transactions.Transaction) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if _, exists := repository.transactions[transaction.TransactionID()]; exists {
		return persistence.ErrAlreadyExists
	}
	if paymentIdentifier := transaction.PaymentIdentifier(); paymentIdentifier != "" {
		if _, exists := repository.paymentIdentifiers[paymentIdentifier]; exists {
			return persistence.ErrPaymentIdentifierConflict
		}
		repository.paymentIdentifiers[paymentIdentifier] = transaction.TransactionID()
	}
	repository.transactions[transaction.TransactionID()] = transaction
	return nil
}

func (repository *TransactionRepository) Get(_ context.Context, transactionID domain.ID) (transactions.Transaction, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	transaction, exists := repository.transactions[transactionID]
	if !exists {
		return transactions.Transaction{}, persistence.ErrNotFound
	}
	return transaction, nil
}

// ListBySeller returns a newest-first cursor page without scanning production data.
func (repository *TransactionRepository) ListBySeller(
	ctx context.Context,
	sellerID domain.ID,
	limit int,
	cursor string,
) ([]transactions.Transaction, *string, error) {
	return repository.QueryBySeller(
		ctx,
		transactions.SellerTransactionQuery{
			SellerID: sellerID,
			Limit:    limit,
			Cursor:   cursor,
		},
	)
}

// QueryBySeller returns a filtered newest-first cursor page.
func (repository *TransactionRepository) QueryBySeller(
	_ context.Context,
	query transactions.SellerTransactionQuery,
) ([]transactions.Transaction, *string, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()

	matching := make([]transactions.Transaction, 0)
	for _, transaction := range repository.transactions {
		if query.Matches(transaction) {
			matching = append(matching, transaction)
		}
	}
	sort.Slice(matching, func(leftIndex, rightIndex int) bool {
		left := matching[leftIndex]
		right := matching[rightIndex]
		if left.CreatedAt().String() == right.CreatedAt().String() {
			return left.TransactionID().String() > right.TransactionID().String()
		}
		return left.CreatedAt().Time().After(right.CreatedAt().Time())
	})

	start := 0
	if query.Cursor != "" {
		cursorID, err := domain.ParseID(query.Cursor, domain.TransactionIDPrefix)
		if err != nil {
			return nil, nil, domain.NewValidationError(
				"cursor",
				"format",
				"must identify a transaction in the seller page",
			)
		}
		start = -1
		for index, transaction := range matching {
			if transaction.TransactionID() == cursorID {
				start = index + 1
				break
			}
		}
		if start < 0 {
			return nil, nil, domain.NewValidationError(
				"cursor",
				"scope",
				"does not belong to this seller",
			)
		}
	}
	end := start + query.Limit
	if end > len(matching) {
		end = len(matching)
	}
	page := append([]transactions.Transaction(nil), matching[start:end]...)
	var nextCursor *string
	if end < len(matching) && len(page) > 0 {
		value := page[len(page)-1].TransactionID().String()
		nextCursor = &value
	}
	return page, nextCursor, nil
}

func (repository *TransactionRepository) Update(_ context.Context, transaction transactions.Transaction, expectedVersion uint64) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	stored, exists := repository.transactions[transaction.TransactionID()]
	if !exists {
		return persistence.ErrNotFound
	}
	if stored.Version() != expectedVersion || transaction.Version() != expectedVersion+1 {
		return persistence.ErrConditionFailed
	}
	if paymentIdentifier := transaction.PaymentIdentifier(); paymentIdentifier != "" && paymentIdentifier != stored.PaymentIdentifier() {
		if _, exists := repository.paymentIdentifiers[paymentIdentifier]; exists {
			return persistence.ErrPaymentIdentifierConflict
		}
		repository.paymentIdentifiers[paymentIdentifier] = transaction.TransactionID()
	}
	repository.transactions[transaction.TransactionID()] = transaction
	return nil
}

func (repository *TransactionRepository) ClaimForwarding(_ context.Context, transactionID domain.ID, expectedVersion uint64, at domain.Timestamp) (transactions.Transaction, bool, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	transaction, exists := repository.transactions[transactionID]
	if !exists {
		return transactions.Transaction{}, false, persistence.ErrNotFound
	}
	if transaction.Version() != expectedVersion || transaction.Status() != transactions.StatusPaymentVerified {
		return transaction, false, nil
	}
	if err := transaction.MarkForwarded(at); err != nil {
		return transactions.Transaction{}, false, err
	}
	repository.transactions[transactionID] = transaction
	return transaction, true, nil
}

type EvidenceRepository struct {
	mutex  sync.RWMutex
	events map[domain.ID][]evidence.Event
}

func NewEvidenceRepository() *EvidenceRepository {
	return &EvidenceRepository{events: make(map[domain.ID][]evidence.Event)}
}

func (repository *EvidenceRepository) Append(_ context.Context, event evidence.Event) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	chain := repository.events[event.TransactionID]
	if event.Sequence != uint64(len(chain)+1) {
		return persistence.ErrConditionFailed
	}
	if len(chain) == 0 {
		if event.PreviousEventHash != nil {
			return persistence.ErrConditionFailed
		}
	} else if event.PreviousEventHash == nil || *event.PreviousEventHash != chain[len(chain)-1].EventHash {
		return persistence.ErrConditionFailed
	}
	repository.events[event.TransactionID] = append(chain, cloneEvent(event))
	return nil
}

func (repository *EvidenceRepository) ListByTransaction(_ context.Context, transactionID domain.ID) ([]evidence.Event, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	chain := repository.events[transactionID]
	events := make([]evidence.Event, len(chain))
	for index, event := range chain {
		events[index] = cloneEvent(event)
	}
	return events, nil
}

type DisputeRepository struct {
	mutex    sync.RWMutex
	disputes map[domain.ID]disputes.Dispute
}

func NewDisputeRepository() *DisputeRepository {
	return &DisputeRepository{disputes: make(map[domain.ID]disputes.Dispute)}
}

func (repository *DisputeRepository) Create(_ context.Context, dispute disputes.Dispute) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if _, exists := repository.disputes[dispute.DisputeID]; exists {
		return persistence.ErrAlreadyExists
	}
	repository.disputes[dispute.DisputeID] = dispute
	return nil
}

func (repository *DisputeRepository) Get(_ context.Context, disputeID domain.ID) (disputes.Dispute, error) {
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	dispute, exists := repository.disputes[disputeID]
	if !exists {
		return disputes.Dispute{}, persistence.ErrNotFound
	}
	return dispute, nil
}

type IdempotencyStore struct {
	mutex   sync.RWMutex
	records map[string]domain.IdempotencyRecord
}

func NewIdempotencyStore() *IdempotencyStore {
	return &IdempotencyStore{records: make(map[string]domain.IdempotencyRecord)}
}

func (store *IdempotencyStore) Load(_ context.Context, scope string, key domain.IdempotencyKey) (domain.IdempotencyRecord, bool, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	record, found := store.records[idempotencyMapKey(scope, key)]
	if found {
		record.ResponseBody = append([]byte(nil), record.ResponseBody...)
	}
	return record, found, nil
}

func (store *IdempotencyStore) SaveIfAbsent(_ context.Context, record domain.IdempotencyRecord) (bool, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	key := idempotencyMapKey(record.Scope, record.Key)
	if _, exists := store.records[key]; exists {
		return false, nil
	}
	record.ResponseBody = append([]byte(nil), record.ResponseBody...)
	store.records[key] = record
	return true, nil
}

func idempotencyMapKey(scope string, key domain.IdempotencyKey) string {
	return scope + "\x00" + string(key)
}

func cloneRoute(route catalog.PaidRoute) catalog.PaidRoute {
	if route.ApprovalThresholdAmount != nil {
		threshold := *route.ApprovalThresholdAmount
		route.ApprovalThresholdAmount = &threshold
	}
	return route
}

func cloneEvent(event evidence.Event) evidence.Event {
	event.Payload = cloneMap(event.Payload)
	if event.PreviousEventHash != nil {
		digest := *event.PreviousEventHash
		event.PreviousEventHash = &digest
	}
	return event
}

func cloneMap(source map[string]any) map[string]any {
	destination := make(map[string]any, len(source))
	for key, value := range source {
		switch typed := value.(type) {
		case map[string]any:
			destination[key] = cloneMap(typed)
		case []any:
			cloned := make([]any, len(typed))
			copy(cloned, typed)
			destination[key] = cloned
		default:
			destination[key] = value
		}
	}
	return destination
}
