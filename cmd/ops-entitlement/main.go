package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
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

	maximumOperationAge = 15 * time.Minute
)

var (
	operationIDPattern = regexp.MustCompile(`^leo_[a-f0-9]{32}$`)
	accountIDPattern   = regexp.MustCompile(`^[0-9]{12}$`)
)

type operationOptions struct {
	OperationID     string
	ProjectName     string
	Environment     string
	AWSAccountID    string
	AWSRegion       string
	TableName       string
	OperatorRoleARN string
	Action          operationAction
	SellerID        domain.ID
	PlanID          billing.PlanID
	AccessEndsAt    time.Time
	EffectiveAt     time.Time
	ExpectedVersion uint64
	PlanSHA256      string
	Confirmation    string
	Apply           bool
}

type callerIdentityClient interface {
	GetCallerIdentity(context.Context, *awssdksts.GetCallerIdentityInput, ...func(*awssdksts.Options)) (*awssdksts.GetCallerIdentityOutput, error)
}

type operationRequest struct {
	SchemaVersion   string          `json:"schemaVersion"`
	OperationID     string          `json:"operationId"`
	Environment     string          `json:"environment"`
	AWSAccountID    string          `json:"awsAccountId"`
	AWSRegion       string          `json:"awsRegion"`
	TableName       string          `json:"tableName"`
	OperatorRoleARN string          `json:"operatorRoleArn"`
	Action          operationAction `json:"action"`
	SellerID        domain.ID       `json:"sellerId"`
	PlanID          billing.PlanID  `json:"planId"`
	PlanVersion     uint64          `json:"planVersion"`
	AccessEndsAt    string          `json:"accessEndsAt,omitempty"`
	EffectiveAt     string          `json:"effectiveAt"`
	ExpectedVersion uint64          `json:"expectedVersion"`
}

type operationPlan struct {
	RequestSHA256 string                            `json:"requestSha256"`
	Current       any                               `json:"current"`
	Proposed      billing.SellerEntitlementSnapshot `json:"proposed"`
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
	effectiveAtValue := flags.String("effective-at", "", "reviewed RFC3339 UTC operation time")
	expectedVersion := flags.Uint64("expected-version", 0, "authoritative current version; zero only for a new entitlement")
	operationID := flags.String("operation-id", "", "seller-scoped leo_<32 lowercase hex> idempotency key")
	projectName := flags.String("project-name", "agentpay", "Terraform project_name value")
	environment := flags.String("environment", "", "Terraform environment: dev, demo, or prod")
	tableName := flags.String("table-name", "", "DynamoDB table from Terraform output")
	region := flags.String("region", "", "AWS region from Terraform output")
	profile := flags.String("profile", "", "optional shared AWS profile")
	accountID := flags.String("account-id", "", "twelve-digit AWS account from Terraform output")
	operatorRoleARN := flags.String("operator-role-arn", "", "Terraform launch_entitlement_operator_role_arn output")
	planSHA256 := flags.String("plan-sha256", "", "reviewed digest printed by the dry run")
	apply := flags.Bool("apply", false, "commit the reviewed operation")
	confirmation := flags.String("confirm", "", "exact confirmation phrase printed by the dry run")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	sellerID, err := domain.ParseID(strings.TrimSpace(*sellerValue), domain.SellerIDPrefix)
	if err != nil {
		return err
	}
	options := operationOptions{
		OperationID: strings.TrimSpace(*operationID), ProjectName: strings.TrimSpace(*projectName),
		Environment: strings.TrimSpace(*environment), AWSAccountID: strings.TrimSpace(*accountID),
		AWSRegion: strings.TrimSpace(*region), TableName: strings.TrimSpace(*tableName),
		OperatorRoleARN: strings.TrimSpace(*operatorRoleARN), Action: operationAction(strings.TrimSpace(*actionValue)),
		SellerID: sellerID, PlanID: billing.PlanID(strings.TrimSpace(*planValue)), ExpectedVersion: *expectedVersion,
		PlanSHA256: strings.TrimSpace(*planSHA256), Confirmation: strings.TrimSpace(*confirmation), Apply: *apply,
	}
	if options.EffectiveAt, err = parseRequiredUTCTime("--effective-at", *effectiveAtValue); err != nil {
		return err
	}
	if strings.TrimSpace(*accessEndsValue) != "" {
		if options.AccessEndsAt, err = parseRequiredUTCTime("--access-ends-at", *accessEndsValue); err != nil {
			return err
		}
	}
	if err := validateOperationOptions(options); err != nil {
		return err
	}
	if err := validateEnvironmentBinding(options.ProjectName, options.Environment, options.AWSAccountID, options.TableName, options.OperatorRoleARN); err != nil {
		return err
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(options.AWSRegion)}
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
	if stringValue(identity.Account) != options.AWSAccountID {
		return errors.New("AWS caller account does not match --account-id")
	}
	if err := validateOperatorCaller(callerARN, options.OperatorRoleARN); err != nil {
		return err
	}

	dynamoClient := awssdkdynamodb.NewFromConfig(configuration)
	if _, err := dynamorepository.NewCatalogRepository(dynamoClient, options.TableName).GetSeller(ctx, sellerID); err != nil {
		return fmt.Errorf("load seller: %w", err)
	}
	repository := dynamorepository.NewSellerEntitlementRepository(dynamoClient, options.TableName)
	storedOperation, err := repository.GetLaunchEntitlementOperation(ctx, sellerID, options.OperationID)
	if err == nil {
		replay, replayErr := validateReplay(storedOperation, options, options.PlanSHA256)
		if replayErr != nil {
			return replayErr
		}
		if options.Apply && options.Confirmation != confirmationPhrase(options, replay.PlanSHA256) {
			return errors.New("--confirm does not match the reviewed operation")
		}
		return printJSON(map[string]any{
			"operationId": replay.OperationID, "sellerId": replay.SellerID,
			"environment": replay.Environment, "planSha256": replay.PlanSHA256,
			"appliedVersion": replay.AppliedVersion, "idempotentReplay": true,
		})
	}
	if !errors.Is(err, persistence.ErrNotFound) {
		return fmt.Errorf("load launch entitlement operation: %w", err)
	}

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
	candidate, err := buildCandidate(options, currentPointer, options.EffectiveAt)
	if err != nil {
		return err
	}
	entitlement, reconciliation, err := billing.ReconcileSellerEntitlement(currentPointer, candidate, domain.NewTimestamp(options.EffectiveAt))
	if err != nil {
		return err
	}
	requestDigest, err := operationRequestDigest(options)
	if err != nil {
		return err
	}
	planDigest, err := operationPlanDigest(requestDigest, currentPointer, entitlement)
	if err != nil {
		return err
	}
	requiredConfirmation := confirmationPhrase(options, planDigest)
	if err := printJSON(map[string]any{
		"schemaVersion": billing.LaunchEntitlementOperationSchemaVersion,
		"operationId":   options.OperationID, "environment": options.Environment,
		"action": options.Action, "sellerId": sellerID, "expectedVersion": options.ExpectedVersion,
		"requestSha256": requestDigest, "planSha256": planDigest,
		"current": currentSnapshot(currentPointer), "proposed": entitlement.Snapshot(),
		"requiredConfirmation": requiredConfirmation, "apply": options.Apply,
	}); err != nil {
		return err
	}
	if !options.Apply {
		return nil
	}
	if options.PlanSHA256 != planDigest {
		return errors.New("--plan-sha256 does not match the current reviewed operation")
	}
	if options.Confirmation != requiredConfirmation {
		return errors.New("--confirm does not match the reviewed operation")
	}
	now := time.Now().UTC()
	if options.EffectiveAt.After(now) || now.Sub(options.EffectiveAt) > maximumOperationAge {
		return errors.New("--effective-at must be within the current 15-minute apply window")
	}
	auditID, err := domain.NewULIDGenerator(nil, nil).New(domain.AuditEventIDPrefix)
	if err != nil {
		return err
	}
	event, err := audit.NewEvent(audit.EventParams{
		AuditEventID: auditID, SellerID: sellerID, ActorType: audit.ActorTypeAdministrator,
		ActorID: callerARN, Action: audit.ActionEntitlementChanged, TargetType: audit.TargetTypeSeller,
		TargetID: sellerID.String(), Outcome: audit.OutcomeSucceeded, RequestID: options.OperationID,
		ChangedFields: []string{"status", "accessEndsAt", "entitlementEpoch", "sourceRevision", "credentialRotationRequired"},
		OccurredAt:    domain.NewTimestamp(now),
	})
	if err != nil {
		return err
	}
	operation := billing.LaunchEntitlementOperation{
		OperationID: options.OperationID, SchemaVersion: billing.LaunchEntitlementOperationSchemaVersion,
		Environment: options.Environment, AWSAccountID: options.AWSAccountID, AWSRegion: options.AWSRegion,
		SellerID: sellerID, Action: string(options.Action), PlanID: entitlement.PlanID(), PlanVersion: entitlement.PlanVersion(),
		EffectiveAt: domain.NewTimestamp(options.EffectiveAt), AccessEndsAt: entitlement.AccessEndsAt(),
		ExpectedVersion: options.ExpectedVersion, AppliedVersion: entitlement.Version(),
		RequestSHA256: requestDigest, PlanSHA256: planDigest, ActorARN: callerARN, AppliedAt: domain.NewTimestamp(now),
	}
	if err := repository.ApplyWithAudit(ctx, entitlement, reconciliation, options.ExpectedVersion, event, operation); err != nil {
		if errors.Is(err, persistence.ErrConditionFailed) {
			storedOperation, replayErr := repository.GetLaunchEntitlementOperation(ctx, sellerID, options.OperationID)
			if replayErr == nil {
				replay, validationErr := validateReplay(storedOperation, options, options.PlanSHA256)
				if validationErr == nil {
					return printJSON(map[string]any{
						"operationId": replay.OperationID, "sellerId": replay.SellerID,
						"environment": replay.Environment, "planSha256": replay.PlanSHA256,
						"appliedVersion": replay.AppliedVersion, "idempotentReplay": true,
					})
				}
			}
		}
		return fmt.Errorf("apply entitlement operation: %w", err)
	}
	fmt.Printf("Applied %s for %s at version %d (operation %s).\n", options.Action, sellerID, entitlement.Version(), options.OperationID)
	return nil
}

func validateOperationOptions(options operationOptions) error {
	if !operationIDPattern.MatchString(options.OperationID) {
		return errors.New("--operation-id must use leo_ followed by 32 lowercase hexadecimal characters")
	}
	if options.Environment != "dev" && options.Environment != "demo" && options.Environment != "prod" {
		return errors.New("--environment must be dev, demo, or prod")
	}
	if !accountIDPattern.MatchString(options.AWSAccountID) {
		return errors.New("--account-id must contain exactly twelve digits")
	}
	if options.AWSRegion == "" || options.TableName == "" || options.OperatorRoleARN == "" {
		return errors.New("--region, --table-name, and --operator-role-arn are required Terraform outputs")
	}
	if options.ProjectName == "" {
		return errors.New("--project-name is required")
	}
	if options.EffectiveAt.IsZero() {
		return errors.New("--effective-at is required")
	}
	if options.Apply && (len(options.PlanSHA256) != 64 || !isLowerHex(options.PlanSHA256)) {
		return errors.New("--apply requires the lowercase --plan-sha256 printed by the dry run")
	}
	return nil
}

func buildCandidate(options operationOptions, current *billing.SellerEntitlement, effectiveAt time.Time) (billing.EntitlementCandidate, error) {
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
		if options.AccessEndsAt.IsZero() || !options.AccessEndsAt.After(effectiveAt) {
			return billing.EntitlementCandidate{}, errors.New("grant requires --access-ends-at after --effective-at")
		}
		_, accessOffsetSeconds := options.AccessEndsAt.Zone()
		_, effectiveOffsetSeconds := effectiveAt.Zone()
		if accessOffsetSeconds != 0 || effectiveOffsetSeconds != 0 {
			return billing.EntitlementCandidate{}, errors.New("operation timestamps must use UTC")
		}
		return billing.EntitlementCandidate{
			SellerID: options.SellerID, PlanID: options.PlanID, PlanVersion: 1,
			Status:             billing.EntitlementStatusActive,
			BillingPeriodStart: domain.NewTimestamp(effectiveAt), BillingPeriodEnd: domain.NewTimestamp(options.AccessEndsAt),
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

func operationRequestDigest(options operationOptions) (string, error) {
	accessEndsAt := ""
	if !options.AccessEndsAt.IsZero() {
		accessEndsAt = options.AccessEndsAt.UTC().Format(time.RFC3339Nano)
	}
	encoded, err := json.Marshal(operationRequest{
		SchemaVersion: billing.LaunchEntitlementOperationSchemaVersion,
		OperationID:   options.OperationID, Environment: options.Environment,
		AWSAccountID: options.AWSAccountID, AWSRegion: options.AWSRegion,
		TableName: options.TableName, OperatorRoleARN: options.OperatorRoleARN,
		Action: options.Action, SellerID: options.SellerID, PlanID: options.PlanID, PlanVersion: 1,
		AccessEndsAt: accessEndsAt, EffectiveAt: options.EffectiveAt.UTC().Format(time.RFC3339Nano),
		ExpectedVersion: options.ExpectedVersion,
	})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func operationPlanDigest(requestDigest string, current *billing.SellerEntitlement, proposed billing.SellerEntitlement) (string, error) {
	encoded, err := json.Marshal(operationPlan{
		RequestSHA256: requestDigest,
		Current:       currentSnapshot(current),
		Proposed:      proposed.Snapshot(),
	})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validateReplay(stored billing.LaunchEntitlementOperation, options operationOptions, suppliedPlanDigest string) (billing.LaunchEntitlementOperation, error) {
	requestDigest, err := operationRequestDigest(options)
	if err != nil {
		return billing.LaunchEntitlementOperation{}, err
	}
	if stored.SchemaVersion != billing.LaunchEntitlementOperationSchemaVersion ||
		stored.OperationID != options.OperationID || stored.Environment != options.Environment ||
		stored.SellerID != options.SellerID || !equalDigest(stored.RequestSHA256, requestDigest) {
		return billing.LaunchEntitlementOperation{}, errors.New("operation ID was already used for a different launch-entitlement request")
	}
	if suppliedPlanDigest != "" && !equalDigest(stored.PlanSHA256, suppliedPlanDigest) {
		return billing.LaunchEntitlementOperation{}, errors.New("--plan-sha256 does not match the previously applied operation")
	}
	return stored, nil
}

func confirmationPhrase(options operationOptions, planDigest string) string {
	return fmt.Sprintf("apply:v%s:%s:%s:%s:%s:%d:%s", billing.LaunchEntitlementOperationSchemaVersion, options.Environment, options.OperationID, options.Action, options.SellerID, options.ExpectedVersion, planDigest)
}

func validateEnvironmentBinding(projectName, environment, accountID, tableName, roleARN string) error {
	resourcePrefix := projectName + "-" + environment
	if tableName != resourcePrefix+"-main" {
		return errors.New("--table-name does not match --project-name and --environment")
	}
	expectedRoleARN := fmt.Sprintf("arn:aws:iam::%s:role/%s-launch-entitlement-operator", accountID, resourcePrefix)
	if roleARN != expectedRoleARN {
		return errors.New("--operator-role-arn does not match the bound account and environment")
	}
	return nil
}

func validateOperatorCaller(callerARN string, roleARN string) error {
	parts := strings.Split(roleARN, ":")
	if len(parts) != 6 || parts[2] != "iam" || !strings.HasPrefix(parts[5], "role/") {
		return errors.New("--operator-role-arn must be the Terraform launch entitlement role ARN")
	}
	roleName := strings.TrimPrefix(parts[5], "role/")
	expectedPrefix := fmt.Sprintf("arn:aws:sts::%s:assumed-role/%s/agentpay-entitlement-", parts[4], roleName)
	if !strings.HasPrefix(callerARN, expectedPrefix) {
		return errors.New("AWS caller must be an assumed session of --operator-role-arn with the required session-name prefix")
	}
	return nil
}

func parseRequiredUTCTime(flagName, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be RFC3339", flagName)
	}
	_, offsetSeconds := parsed.Zone()
	if offsetSeconds != 0 || !strings.HasSuffix(strings.TrimSpace(value), "Z") {
		return time.Time{}, fmt.Errorf("%s must use UTC with a Z suffix", flagName)
	}
	return parsed.UTC(), nil
}

func currentSnapshot(current *billing.SellerEntitlement) any {
	if current == nil {
		return nil
	}
	return current.Snapshot()
}

func equalDigest(left, right string) bool {
	return len(left) == 64 && len(right) == 64 && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func isLowerHex(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func printJSON(value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
