package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
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

var ErrNotFound = errors.New("storage: object not found")

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

func (c *Client) Head(ctx context.Context, key string) (contentLength int64, contentType string, err error) {
	out, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return 0, "", ErrNotFound
		}

		return 0, "", fmt.Errorf("storage: head %q: %w", key, err)
	}

	if out.ContentLength != nil {
		contentLength = *out.ContentLength
	}

	if out.ContentType != nil {
		contentType = *out.ContentType
	}

	return contentLength, contentType, nil
}

func (c *Client) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage: get %q: %w", key, err)
	}

	return out.Body, nil
}

func (c *Client) Put(ctx context.Context, key string, body io.Reader) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   body,
	})
	if err != nil {
		return fmt.Errorf("storage: put %q: %w", key, err)
	}

	return nil
}

func isNotFound(err error) bool {
	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		switch apiError.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return true
		}
	}

	return false
}
