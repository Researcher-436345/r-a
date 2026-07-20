from __future__ import annotations

import logging

import boto3
from botocore.client import Config
from botocore.exceptions import ClientError

from app.core.config import settings

logger = logging.getLogger(__name__)

_bucket_ready = False


def get_s3_client(*, public: bool = False):
    endpoint = settings.s3_public_endpoint_url if public else settings.s3_endpoint_url
    return boto3.client(
        "s3",
        endpoint_url=endpoint,
        aws_access_key_id=settings.s3_access_key,
        aws_secret_access_key=settings.s3_secret_key,
        region_name=settings.s3_region,
        config=Config(signature_version="s3v4"),
    )


def ensure_bucket() -> None:
    global _bucket_ready
    if _bucket_ready:
        return

    client = get_s3_client()
    bucket = settings.s3_bucket
    existing = {item["Name"] for item in client.list_buckets().get("Buckets", [])}
    if bucket not in existing:
        client.create_bucket(Bucket=bucket)

    # MinIO may not support PutBucketCors via S3 API — don't fail uploads because of this.
    try:
        client.put_bucket_cors(
            Bucket=bucket,
            CORSConfiguration={
                "CORSRules": [
                    {
                        "AllowedHeaders": ["*"],
                        "AllowedMethods": ["GET", "HEAD"],
                        "AllowedOrigins": [
                            "http://localhost:5173",
                            "http://127.0.0.1:5173",
                        ],
                        "ExposeHeaders": ["ETag"],
                        "MaxAgeSeconds": 3000,
                    }
                ]
            },
        )
    except ClientError as exc:
        logger.warning("Could not set bucket CORS (non-fatal): %s", exc)

    _bucket_ready = True


def upload_bytes(key: str, data: bytes, content_type: str = "application/pdf") -> None:
    ensure_bucket()
    get_s3_client().put_object(
        Bucket=settings.s3_bucket,
        Key=key,
        Body=data,
        ContentType=content_type,
    )


def download_bytes(key: str) -> bytes:
    response = get_s3_client().get_object(Bucket=settings.s3_bucket, Key=key)
    return response["Body"].read()


def presigned_get_url(key: str) -> str:
    return get_s3_client(public=True).generate_presigned_url(
        "get_object",
        Params={"Bucket": settings.s3_bucket, "Key": key},
        ExpiresIn=settings.s3_presign_expire_seconds,
    )
