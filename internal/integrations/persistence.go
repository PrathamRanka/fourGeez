package integrations

import "github.com/fourgeez/agentpay/internal/domain"

// Snapshot is the explicit persisted representation of a credential.
type Snapshot struct {
	CredentialID domain.ID         `json:"credentialId"`
	SellerID     domain.ID         `json:"sellerId"`
	TokenHash    string            `json:"tokenHash"`
	Label        string            `json:"label"`
	Scopes       []Scope           `json:"scopes"`
	ExpiresAt    *domain.Timestamp `json:"expiresAt"`
	RevokedAt    *domain.Timestamp `json:"revokedAt"`
	CreatedAt    domain.Timestamp  `json:"createdAt"`
	UpdatedAt    domain.Timestamp  `json:"updatedAt"`
	Version      uint64            `json:"version"`
}

// Snapshot returns an isolated persisted representation.
func (credential Credential) Snapshot() Snapshot {
	return Snapshot{
		CredentialID: credential.CredentialID(),
		SellerID:     credential.SellerID(),
		TokenHash:    credential.TokenHash(),
		Label:        credential.Label(),
		Scopes:       credential.Scopes(),
		ExpiresAt:    credential.ExpiresAt(),
		RevokedAt:    credential.RevokedAt(),
		CreatedAt:    credential.CreatedAt(),
		UpdatedAt:    credential.UpdatedAt(),
		Version:      credential.Version(),
	}
}

// RestoreCredential rebuilds a credential from trusted persisted state.
func RestoreCredential(snapshot Snapshot) Credential {
	return Credential{
		credentialID: snapshot.CredentialID,
		sellerID:     snapshot.SellerID,
		tokenHash:    snapshot.TokenHash,
		label:        snapshot.Label,
		scopes:       append([]Scope(nil), snapshot.Scopes...),
		expiresAt:    copyTimestamp(snapshot.ExpiresAt),
		revokedAt:    copyTimestamp(snapshot.RevokedAt),
		createdAt:    snapshot.CreatedAt,
		updatedAt:    snapshot.UpdatedAt,
		version:      snapshot.Version,
	}
}
