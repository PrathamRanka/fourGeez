package sellerworkspace

import (
	"context"
	"testing"
)

func TestAuthenticatedAccountVerificationTrustsOnlyAnAuthenticatedSubject(t *testing.T) {
	t.Parallel()
	verification := AuthenticatedAccountVerification{}
	verified, err := verification.EmailVerified(context.Background(), "cognito-subject")
	if err != nil || !verified {
		t.Fatalf("EmailVerified() = %t, %v", verified, err)
	}
	verified, err = verification.EmailVerified(context.Background(), "")
	if err == nil || verified {
		t.Fatalf("empty subject EmailVerified() = %t, %v", verified, err)
	}
}
