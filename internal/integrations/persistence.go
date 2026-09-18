package integrations

import "github.com/fourgeez/agentpay/internal/domain"

// Snapshot is the explicit persisted representation of a credential.
type Snapshot struct {
	CredentialID           domain.ID         `json:"credentialId"`
	SellerID               domain.ID         `json:"sellerId"`
	TokenHash              string            `json:"tokenHash"`
	Label                  string            `json:"label"`
	Scopes                 []Scope           `json:"scopes"`
	EntitlementEpoch       uint64            `json:"entitlementEpoch"`
	ExpiresAt              *domain.Timestamp `json:"expiresAt"`
	RevokedAt              *domain.Timestamp `json:"revokedAt"`
	LastUsedAt             *domain.Timestamp `json:"lastUsedAt"`
	ReplacedByCredentialID *domain.ID        `json:"replacedByCredentialId"`
	CreatedAt              domain.Timestamp  `json:"createdAt"`
	UpdatedAt              domain.Timestamp  `json:"updatedAt"`
	Version                uint64            `json:"version"`
}

// Snapshot returns an isolated persisted representation.
func (credential Credential) Snapshot() Snapshot {
	return Snapshot{
		CredentialID:           credential.CredentialID(),
		SellerID:               credential.SellerID(),
		TokenHash:              credential.TokenHash(),
		Label:                  credential.Label(),
		Scopes:                 credential.Scopes(),
		EntitlementEpoch:       credential.EntitlementEpoch(),
		ExpiresAt:              credential.ExpiresAt(),
		RevokedAt:              credential.RevokedAt(),
		LastUsedAt:             credential.LastUsedAt(),
		ReplacedByCredentialID: credential.ReplacedByCredentialID(),
		CreatedAt:              credential.CreatedAt(),
		UpdatedAt:              credential.UpdatedAt(),
		Version:                credential.Version(),
	}
}

// RestoreCredential rebuilds a credential from trusted persisted state.
func RestoreCredential(snapshot Snapshot) Credential {
	return Credential{
		credentialID:           snapshot.CredentialID,
		sellerID:               snapshot.SellerID,
		tokenHash:              snapshot.TokenHash,
		label:                  snapshot.Label,
		scopes:                 append([]Scope(nil), snapshot.Scopes...),
		entitlementEpoch:       snapshot.EntitlementEpoch,
		expiresAt:              copyTimestamp(snapshot.ExpiresAt),
		revokedAt:              copyTimestamp(snapshot.RevokedAt),
		lastUsedAt:             copyTimestamp(snapshot.LastUsedAt),
		replacedByCredentialID: copyID(snapshot.ReplacedByCredentialID),
		createdAt:              snapshot.CreatedAt,
		updatedAt:              snapshot.UpdatedAt,
		version:                snapshot.Version,
	}
}

func copyID(identifier *domain.ID) *domain.ID {
	if identifier == nil {
		return nil
	}
	copy := *identifier
	return &copy
}
