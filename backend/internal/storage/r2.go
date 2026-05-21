package storage

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type R2 struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	publicBase    string
}

func NewR2(accountID, accessKey, secretKey, bucket, publicBase string) (*R2, error) {
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	cl := s3.New(s3.Options{
		Region:       "auto",
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: true,
	})
	return &R2{
		client:        cl,
		presignClient: s3.NewPresignClient(cl),
		bucket:        bucket,
		publicBase:    publicBase,
	}, nil
}

// PresignPut returns a URL the client can PUT to directly, valid for ttl.
// publicURL is what the client should send back to us as input_image_url.
func (r *R2) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (uploadURL, publicURL string, err error) {
	out, err := r.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", "", err
	}
	return out.URL, r.publicBase + "/" + key, nil
}

// Upload writes raw bytes from the backend (used for AI-generated outputs).
// Returns the public URL.
func (r *R2) Upload(ctx context.Context, key, contentType string, data []byte) (string, error) {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
		Body:        bytes.NewReader(data),
	})
	if err != nil {
		return "", err
	}
	return r.publicBase + "/" + key, nil
}
