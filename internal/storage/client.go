package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Client struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
}

type R2Config struct {
	AccountID    string
	AccessKey    string
	AccessSecret string
	Bucket       string
}

func NewR2(ctx context.Context, cfg R2Config) (*Client, error) {
	r2Cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.AccessSecret, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("storage: failed to load storage config: %w", err)
	}

	client := s3.NewFromConfig(r2Cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID))
	})

	presignClient := s3.NewPresignClient(client)

	return &Client{
		client:    client,
		presigner: presignClient,
		bucket:    cfg.Bucket,
	}, nil
}

func (c *Client) PresignUpload(ctx context.Context, key, contentType string, contentLength int64, expiry time.Duration) (string, error) {
	req, err := c.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(contentLength),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presignUpload: presign put failed: %w", err)
	}

	return req.URL, nil
}
