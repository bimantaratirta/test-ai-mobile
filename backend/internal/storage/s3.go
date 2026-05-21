package storage

import (
	"bytes"
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3 wraps any S3-compatible object storage (IDCloudHost, Cloudflare R2,
// AWS S3, MinIO, etc.). Endpoint is the full base URL of the S3 API.
type S3 struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	publicBase    string
}

func NewS3(endpoint, region, accessKey, secretKey, bucket, publicBase string) (*S3, error) {
	if region == "" {
		region = "auto"
	}
	cl := s3.New(s3.Options{
		Region:       region,
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: true,
	})
	return &S3{
		client:        cl,
		presignClient: s3.NewPresignClient(cl),
		bucket:        bucket,
		publicBase:    publicBase,
	}, nil
}

// PresignPut returns a URL the client can PUT to directly, valid for ttl.
// publicURL is what the client should send back as input_image_url.
func (r *S3) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (uploadURL, publicURL string, err error) {
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

// Upload writes raw bytes (used for AI-generated outputs). Returns public URL.
func (r *S3) Upload(ctx context.Context, key, contentType string, data []byte) (string, error) {
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
