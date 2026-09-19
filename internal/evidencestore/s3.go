package evidencestore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/evidence"
	"github.com/fourgeez/agentpay/internal/persistence"
)

const maximumEvidenceObjectBytes = 1 << 20
const maximumEvidenceObjects = 1000

type Client interface {
	PutObject(context.Context, *awss3.PutObjectInput, ...func(*awss3.Options)) (*awss3.PutObjectOutput, error)
	ListObjectsV2(context.Context, *awss3.ListObjectsV2Input, ...func(*awss3.Options)) (*awss3.ListObjectsV2Output, error)
	GetObject(context.Context, *awss3.GetObjectInput, ...func(*awss3.Options)) (*awss3.GetObjectOutput, error)
}

type S3Repository struct {
	client Client
	bucket string
}

func NewS3Repository(client Client, bucket string) (*S3Repository, error) {
	bucket = strings.TrimSpace(bucket)
	if client == nil || bucket == "" {
		return nil, errors.New("S3 evidence repository requires client and bucket")
	}
	return &S3Repository{client: client, bucket: bucket}, nil
}

func (repository *S3Repository) Append(ctx context.Context, event evidence.Event) error {
	encoded, err := encodeEvent(event)
	if err != nil {
		return err
	}
	key := evidenceObjectKey(event.TransactionID, event.Sequence)
	_, err = repository.client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket: aws.String(repository.bucket), Key: aws.String(key), Body: bytes.NewReader(encoded),
		ContentType: aws.String("application/json"), IfNoneMatch: aws.String("*"),
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
		Metadata:          map[string]string{"event-id": event.EventID.String(), "event-hash": event.EventHash.String()},
	})
	if isPreconditionFailure(err) {
		return persistence.ErrConditionFailed
	}
	return err
}

func (repository *S3Repository) ListByTransaction(ctx context.Context, transactionID domain.ID) ([]evidence.Event, error) {
	prefix := evidenceObjectPrefix(transactionID)
	output, err := repository.client.ListObjectsV2(ctx, &awss3.ListObjectsV2Input{
		Bucket: aws.String(repository.bucket), Prefix: aws.String(prefix), MaxKeys: aws.Int32(maximumEvidenceObjects),
	})
	if err != nil {
		return nil, err
	}
	if aws.ToBool(output.IsTruncated) {
		return nil, errors.New("evidence chain exceeds supported object limit")
	}
	keys := make([]string, 0, len(output.Contents))
	for _, object := range output.Contents {
		key := aws.ToString(object.Key)
		if strings.HasPrefix(key, prefix) && strings.HasSuffix(key, ".json") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	events := make([]evidence.Event, 0, len(keys))
	for _, key := range keys {
		output, err := repository.client.GetObject(ctx, &awss3.GetObjectInput{Bucket: aws.String(repository.bucket), Key: aws.String(key)})
		if err != nil {
			return nil, err
		}
		event, readErr := decodeEvent(output.Body)
		closeErr := output.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		events = append(events, event)
	}
	for index, event := range events {
		if event.TransactionID != transactionID || event.Sequence != uint64(index+1) {
			return nil, evidence.ErrChainInvalid
		}
	}
	return events, nil
}

func encodeEvent(event evidence.Event) ([]byte, error) {
	return json.Marshal(event)
}

func decodeEvent(reader io.Reader) (evidence.Event, error) {
	limited := io.LimitReader(reader, maximumEvidenceObjectBytes+1)
	encoded, err := io.ReadAll(limited)
	if err != nil {
		return evidence.Event{}, err
	}
	if len(encoded) > maximumEvidenceObjectBytes {
		return evidence.Event{}, errors.New("evidence object exceeds maximum size")
	}
	var event evidence.Event
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		return evidence.Event{}, fmt.Errorf("decode evidence object: %w", err)
	}
	return event, nil
}

func evidenceObjectPrefix(transactionID domain.ID) string {
	return "transactions/" + transactionID.String() + "/events/"
}

func evidenceObjectKey(transactionID domain.ID, sequence uint64) string {
	return fmt.Sprintf("%s%06d.json", evidenceObjectPrefix(transactionID), sequence)
}

func isPreconditionFailure(err error) bool {
	if err == nil {
		return false
	}
	var apiError smithy.APIError
	return errors.As(err, &apiError) && (apiError.ErrorCode() == "PreconditionFailed" || apiError.ErrorCode() == "ConditionalRequestConflict")
}

var _ evidence.Repository = (*S3Repository)(nil)
