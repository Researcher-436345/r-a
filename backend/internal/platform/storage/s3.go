package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	internal *minio.Client
	public   *minio.Client
	bucket   string
	expire   time.Duration
}

func New(endpoint, publicEndpoint, accessKey, secretKey, bucket, region string, expire time.Duration) (*Client, error) {
	internal, err := newMinio(endpoint, accessKey, secretKey, region)
	if err != nil {
		return nil, err
	}
	public, err := newMinio(publicEndpoint, accessKey, secretKey, region)
	if err != nil {
		return nil, err
	}
	return &Client{internal: internal, public: public, bucket: bucket, expire: expire}, nil
}

func newMinio(rawEndpoint, accessKey, secretKey, region string) (*minio.Client, error) {
	u, err := url.Parse(rawEndpoint)
	if err != nil {
		return nil, err
	}
	secure := u.Scheme == "https"
	host := u.Host
	if host == "" {
		host = strings.TrimPrefix(strings.TrimPrefix(rawEndpoint, "https://"), "http://")
		secure = strings.HasPrefix(rawEndpoint, "https://")
	}
	return minio.New(host, &minio.Options{
		Creds:        credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:       secure,
		Region:       region,
		BucketLookup: minio.BucketLookupPath,
	})
}

func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.internal.BucketExists(ctx, c.bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := c.internal.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Upload(ctx context.Context, key string, data []byte) error {
	_, err := c.internal.PutObject(ctx, c.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/pdf",
	})
	return err
}

func (c *Client) Download(ctx context.Context, key string) ([]byte, error) {
	obj, err := c.internal.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

func (c *Client) PresignGet(ctx context.Context, key string) (string, time.Duration, error) {
	u, err := c.public.PresignedGetObject(ctx, c.bucket, key, c.expire, nil)
	if err != nil {
		return "", 0, err
	}
	return u.String(), c.expire, nil
}

func PDFKey(paperID, hex string) string {
	return fmt.Sprintf("papers/%s/%s.pdf", paperID, hex)
}
