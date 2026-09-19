package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awssdkdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awssdksts "github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/fourgeez/agentpay/internal/audit"
	"github.com/fourgeez/agentpay/internal/billing"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
	dynamorepository "github.com/fourgeez/agentpay/internal/persistence/dynamodb"
)

type operationAction string

const (
	actionGrant   operationAction = "grant"
	actionSuspend operationAction = "suspend"
	actionCancel  operationAction = "cancel"
)

type operationOptions struct {
	Action          operationAction
	SellerID        domain.ID
	PlanID          billing.PlanID
	AccessEndsAt    time.Time
	ExpectedVersion uint64
	Confirmation    string
	Apply           bool
}

type callerIdentityClient interface {
	GetCallerIdentity(context.Context, *awssdksts.GetCallerIdentityInput, ...func(*awssdksts.Options)) (*awssdksts.GetCallerIdentityOutput, error)
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("ops-entitlement", flag.ContinueOnError)
	actionValue := flags.String("action", "", "grant, suspend, or cancel")
	sellerValue := flags.String("seller-id", "", "seller identifier")
	planValue := flags.String("plan", "starter", "starter, growth, or scale")
	accessEndsValue := flags.String("access-ends-at", "", "exclusive RFC3339 UTC access boundary for grant")
	expectedVersion := flags.Uint64("expected-version", 0, "authoritative current version; zero only for a new entitlement")
	tableName := flags.String("table-name", "", "DynamoDB table from Terraform output")
	region := flags.String("region", "ap-south-1", "AWS region")
	profile := flags.String("profile", "", "optional shared AWS profile")
	accountID := flags.String("account-id", "", "expected twelve-digit AWS account")
	operatorRoleARN := flags.String("operator-role-arn", "", "Terraform launch_entitlement_operator_role_arn output")
	apply := flags.Bool("apply", false, "commit the reviewed operation")
	confirmation := flags.String("confirm", "", "exact action:sellerId:expectedVersion phrase")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if strings.TrimSpace(*tableName) == "" {
		return errors.New("--table-name is required and must come from Terraform output")
	}
	sellerID, err := domain.ParseID(strings.TrimSpace(*sellerValue), domain.SellerIDPrefix)
	if err != nil {
		return err
	}
	options := operationOptions{
		Action: operationAction(strings.TrimSpace(*actionValue)), SellerID: sellerID,
		PlanID: billing.PlanID(strings.TrimSpace(*planValue)), ExpectedVersion: *expectedVersion,
		Confirmation: strings.TrimSpace(*confirmation), Apply: *apply,
	}
	if *accessEndsValue != "" {
		options.AccessEndsAt, err = time.Parse(time.RFC3339, *accessEndsValue)
		if err != nil {
			return errors.New("--access-ends-at must be RFC3339")
		}
	}
	loadOptions := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(*region)}
	if strings.TrimSpace(*profile) != "" {
		loadOptions = append(loadOptions, awsconfig.WithSharedConfigProfile(strings.TrimSpace(*profile)))
	}
	configuration, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return fmt.Errorf("load AWS configuration: %w", err)
	}
	identity, err := awssdksts.NewFromConfig(configuration).GetCallerIdentity(ctx, &awssdksts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("resolve AWS caller identity: %w", err)
	}
	callerARN := strings.TrimSpace(stringValue(identity.Arn))
	if callerARN == "" || strings.HasSuffix(callerARN, ":root") {
		return errors.New("manual entitlement operations require a non-root assumed role or IAM principal")
	}
	if *accountID != "" && stringValue(identity.Account) != strings.TrimSpace(*accountID) {
		return errors.New("AWS caller account does not match --account-id")
	}
	if options.Apply {
		if err := validateOperatorCaller(callerARN, strings.TrimSpace(*operatorRoleARN)); err != nil {
			return err
		}
	}
	dynamoClient := awssdkdynamodb.NewFromConfig(configuration)
	if _, err := dynamorepository.NewCatalogRepository(dynamoClient, *tableName).GetSeller(ctx, sellerID); err != nil {
		return fmt.Errorf("load seller: %w", err)
	}
	repository := dynamorepository.NewSellerEntitlementRepository(dynamoClient, *tableName)
	current, err := repository.Get(ctx, sellerID)
	if errors.Is(err, persistence.ErrNotFound) || errors.Is(err, billing.ErrSellerEntitlementNotFound) {
		current = billing.SellerEntitlement{}
	} else if err != nil {
		return fmt.Errorf("load current entitlement: %w", err)
	}
	var currentPointer *billing.SellerEntitlement
	if current.SellerID() != "" {
		currentPointer = &current
	}
	now := time.Now().UTC()
	candidate, err := buildCandidate(options, currentPointer, now)
	if err != nil {
		return err
	}
	entitlement, reconciliation, err := billing.ReconcileSellerEntitlement(currentPointer, candidate, domain.NewTimestamp(now))
	if err != nil {
		return err
	}
	plan := map[string]any{
		"action": options.Action, "sellerId": sellerID, "expectedVersion": options.ExpectedVersion,
		"current": currentSnapshot(currentPointer), "proposed": entitlement.Snapshot(),
		"requiredConfirmation": confirmationPhrase(options.Action, sellerID, options.ExpectedVersion),
		"apply":                options.Apply,
	}
	encoded, _ := json.MarshalIndent(plan, "", "  ")
	fmt.Println(string(encoded))
	if !options.Apply {
		return nil
	}
	if options.Confirmation != confirmationPhrase(options.Action, sellerID, options.ExpectedVersion) {
		return errors.New("--confirm does not match the reviewed operation")
	}
	auditID, err := domain.NewULIDGenerator(nil, nil).New(domain.AuditEventIDPrefix)
	if err != nil {
		return err
	}
	requestID, err := newOperationRequestID()
	if err != nil {
		return err
	}
	event, err := audit.NewEvent(audit.EventParams{
		AuditEventID: auditID, SellerID: sellerID, ActorType: audit.ActorTypeAdministrator,
		ActorID: callerARN, Action: audit.ActionEntitlementChanged, TargetType: audit.TargetTypeSeller,
		TargetID: sellerID.String(), Outcome: audit.OutcomeSucceeded, RequestID: requestID,
		ChangedFields: []string{"status", "accessEndsAt", "entitlementEpoch", "sourceRevision", "credentialRotationRequired"},
		OccurredAt:    domain.NewTimestamp(now),
	})
	if err != nil {
		return err
	}
	if err := repository.ApplyWithAudit(ctx, entitlement, reconciliation, options.ExpectedVersion, event); err != nil {
		return fmt.Errorf("apply entitlement and audit event: %w", err)
	}
	fmt.Printf("Applied %s for %s at version %d.\n", options.Action, sellerID, entitlement.Version())
	return nil
}

func buildCandidate(options operationOptions, current *billing.SellerEntitlement, now time.Time) (billing.EntitlementCandidate, error) {
	if options.Action != actionGrant && options.Action != actionSuspend && options.Action != actionCancel {
		return billing.EntitlementCandidate{}, errors.New("--action must be grant, suspend, or cancel")
	}
	if current == nil && options.ExpectedVersion != 0 {
		return billing.EntitlementCandidate{}, errors.New("new entitlement requires --expected-version 0")
	}
	if current != nil && current.Version() != options.ExpectedVersion {
		return billing.EntitlementCandidate{}, fmt.Errorf("expected version %d does not match current version %d", options.ExpectedVersion, current.Version())
	}
	if options.Action == actionGrant {
		if options.AccessEndsAt.IsZero() || !options.AccessEndsAt.After(now) {
			return billing.EntitlementCandidate{}, errors.New("grant requires a future --access-ends-at")
		}
		_, offsetSeconds := options.AccessEndsAt.Zone()
		if offsetSeconds != 0 {
			return billing.EntitlementCandidate{}, errors.New("--access-ends-at must use UTC")
		}
		return billing.EntitlementCandidate{
			SellerID: options.SellerID, PlanID: options.PlanID, PlanVersion: 1,
			Status:             billing.EntitlementStatusActive,
			BillingPeriodStart: domain.NewTimestamp(now), BillingPeriodEnd: domain.NewTimestamp(options.AccessEndsAt),
			AccessEndsAt: domain.NewTimestamp(options.AccessEndsAt),
			Source:       billing.EntitlementSourceOperator, Provider: billing.EntitlementProviderOperator,
		}, nil
	}
	if current == nil {
		return billing.EntitlementCandidate{}, errors.New("suspend and cancel require an existing entitlement")
	}
	status := billing.EntitlementStatusSuspended
	reason := billing.EntitlementStatusReasonAdministrative
	if options.Action == actionCancel {
		status = billing.EntitlementStatusCancelled
		reason = billing.EntitlementStatusReasonCancelled
	}
	return billing.EntitlementCandidate{
		SellerID: current.SellerID(), PlanID: current.PlanID(), PlanVersion: current.PlanVersion(), Status: status,
		BillingPeriodStart: current.BillingPeriodStart(), BillingPeriodEnd: current.BillingPeriodEnd(), AccessEndsAt: current.AccessEndsAt(),
		Source: billing.EntitlementSourceOperator, StatusReason: reason, Provider: billing.EntitlementProviderOperator,
	}, nil
}

func confirmationPhrase(action operationAction, sellerID domain.ID, expectedVersion uint64) string {
	return fmt.Sprintf("%s:%s:%d", action, sellerID, expectedVersion)
}

func currentSnapshot(current *billing.SellerEntitlement) any {
	if current == nil {
		return nil
	}
	return current.Snapshot()
}

func newOperationRequestID() (string, error) {
	value := make([]byte, 16)
	if _, err := cryptorand.Read(value); err != nil {
		return "", err
	}
	return "ops_" + hex.EncodeToString(value), nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func validateOperatorCaller(callerARN string, roleARN string) error {
	parts := strings.Split(roleARN, ":")
	if len(parts) != 6 || parts[2] != "iam" || !strings.HasPrefix(parts[5], "role/") {
		return errors.New("--operator-role-arn must be the Terraform launch entitlement role ARN")
	}
	roleName := strings.TrimPrefix(parts[5], "role/")
	expectedPrefix := fmt.Sprintf("arn:aws:sts::%s:assumed-role/%s/", parts[4], roleName)
	if !strings.HasPrefix(callerARN, expectedPrefix) {
		return errors.New("AWS caller must be an assumed session of --operator-role-arn")
	}
	return nil
}
