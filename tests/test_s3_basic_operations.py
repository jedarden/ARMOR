#!/usr/bin/env python3
"""End-to-end boto3 coverage for the documented ARMOR S3 API.

The compatibility gate starts a built ARMOR image and sets the
``ARMOR_COMPAT_*`` variables before running this file.  Keeping the client in
Python is intentional: the test must exercise botocore's SigV4 signing and
the boto3 transfer manager, rather than a hand-written signer or another SDK.
"""

import io
import os
import unittest
import uuid

import boto3
from boto3.s3.transfer import TransferConfig
from botocore.config import Config
from botocore.exceptions import ClientError


class Boto3S3CompatibilityTest(unittest.TestCase):
    """Exercise the S3 operations promised by README.md against a live image."""

    @classmethod
    def setUpClass(cls):
        cls.endpoint = os.environ.get("ARMOR_COMPAT_ENDPOINT")
        cls.access_key = os.environ.get("ARMOR_COMPAT_ACCESS_KEY")
        cls.secret_key = os.environ.get("ARMOR_COMPAT_SECRET_KEY")
        cls.bucket = os.environ.get("ARMOR_BUCKET")
        cls.region = os.environ.get("ARMOR_COMPAT_REGION", "us-east-1")

        missing = [
            name
            for name, value in (
                ("ARMOR_COMPAT_ENDPOINT", cls.endpoint),
                ("ARMOR_COMPAT_ACCESS_KEY", cls.access_key),
                ("ARMOR_COMPAT_SECRET_KEY", cls.secret_key),
                ("ARMOR_BUCKET", cls.bucket),
            )
            if not value
        ]
        if missing:
            raise RuntimeError(
                "boto3 compatibility requires " + ", ".join(missing)
            )

        # Path-style addressing is required for ARMOR's single S3 endpoint.
        # signature_version=s3v4 makes the authentication assertion explicit;
        # it also prevents a local AWS configuration from selecting SigV2.
        cls.client = boto3.client(
            "s3",
            endpoint_url=cls.endpoint,
            aws_access_key_id=cls.access_key,
            aws_secret_access_key=cls.secret_key,
            region_name=cls.region,
            config=Config(
                signature_version="s3v4",
                s3={"addressing_style": "path"},
                retries={"max_attempts": 3, "mode": "standard"},
            ),
        )
        cls.prefix = "compat/boto3/" + uuid.uuid4().hex
        cls.single_key = cls.prefix + "/single.bin"
        cls.multipart_key = cls.prefix + "/multipart.bin"
        cls.delete_key = cls.prefix + "/delete-me.bin"
        cls.overwrite_key = cls.prefix + "/overwrite.bin"
        cls._ensure_bucket()

    @classmethod
    def _ensure_bucket(cls):
        """Create the configured test bucket when the image starts empty."""
        try:
            cls.client.create_bucket(Bucket=cls.bucket)
        except ClientError as error:
            code = error.response.get("Error", {}).get("Code")
            if code not in {"BucketAlreadyOwnedByYou", "BucketAlreadyExists"}:
                raise

    @classmethod
    def tearDownClass(cls):
        if not hasattr(cls, "client"):
            return
        keys = (
            cls.single_key,
            cls.multipart_key,
            cls.delete_key,
            cls.overwrite_key,
        )
        for key in keys:
            if key:
                try:
                    cls.client.delete_object(Bucket=cls.bucket, Key=key)
                except Exception:
                    # Preserve the original assertion if cleanup encounters a
                    # server-side failure. The gate uses a throwaway bucket.
                    pass

    def test_authentication_rejects_invalid_credentials(self):
        """SigV4 rejects a wrong secret and an unknown access key."""
        cases = {
            "wrong-secret": (
                self.access_key,
                self.secret_key + "-tampered",
                "SignatureDoesNotMatch",
            ),
            "unknown-access-key": (
                self.access_key + "-tampered",
                self.secret_key,
                "InvalidAccessKeyId",
            ),
        }
        for label, (access_key, secret_key, error_code) in cases.items():
            with self.subTest(case=label):
                intruder = boto3.client(
                    "s3",
                    endpoint_url=self.endpoint,
                    aws_access_key_id=access_key,
                    aws_secret_access_key=secret_key,
                    region_name=self.region,
                    config=Config(
                        signature_version="s3v4",
                        s3={"addressing_style": "path"},
                    ),
                )
                with self.assertRaises(ClientError) as ctx:
                    intruder.list_buckets()
                response = ctx.exception.response
                self.assertEqual(
                    response["ResponseMetadata"]["HTTPStatusCode"], 403
                )
                self.assertEqual(response["Error"]["Code"], error_code)

    def test_delete_removes_object(self):
        """DeleteObject removes an object from reads and listings."""
        # Seed a bare-prefix list cache entry before writing. boto3 commonly
        # lists without a trailing slash, so a write must invalidate that exact
        # query shape rather than only the object's directory.
        before = self.client.list_objects_v2(Bucket=self.bucket, Prefix=self.prefix)
        self.assertNotIn(
            self.delete_key,
            {entry["Key"] for entry in before.get("Contents", [])},
        )
        self.client.put_object(
            Bucket=self.bucket,
            Key=self.delete_key,
            Body=b"to-be-deleted" * 100,
        )
        after_put = self.client.list_objects_v2(
            Bucket=self.bucket, Prefix=self.prefix
        )
        self.assertIn(
            self.delete_key,
            {entry["Key"] for entry in after_put.get("Contents", [])},
        )

        self.client.delete_object(Bucket=self.bucket, Key=self.delete_key)

        with self.assertRaises(ClientError) as head_ctx:
            self.client.head_object(Bucket=self.bucket, Key=self.delete_key)
        self.assertEqual(
            head_ctx.exception.response["ResponseMetadata"]["HTTPStatusCode"], 404
        )
        with self.assertRaises(ClientError) as get_ctx:
            self.client.get_object(Bucket=self.bucket, Key=self.delete_key)
        self.assertEqual(get_ctx.exception.response["Error"]["Code"], "NoSuchKey")
        listed = self.client.list_objects_v2(Bucket=self.bucket, Prefix=self.prefix)
        self.assertNotIn(
            self.delete_key,
            {entry["Key"] for entry in listed.get("Contents", [])},
        )

        # S3 deletes are idempotent.
        gone = self.client.delete_object(Bucket=self.bucket, Key=self.delete_key)
        self.assertEqual(gone["ResponseMetadata"]["HTTPStatusCode"], 204)

    def test_documented_s3_api(self):
        """Verify SigV4, bucket/object operations, ranges, metadata, and multipart."""
        # Bucket operations. The compatibility image may start with an empty
        # filesystem backend, so create the configured test bucket when it is
        # not already present. Existing deployments are allowed to report the
        # normal S3 already-owned response.
        try:
            self.client.create_bucket(Bucket=self.bucket)
        except ClientError as error:
            code = error.response.get("Error", {}).get("Code")
            self.assertIn(code, {"BucketAlreadyOwnedByYou", "BucketAlreadyExists"})
        buckets = self.client.list_buckets()["Buckets"]
        self.assertIn(self.bucket, {entry["Name"] for entry in buckets})
        self.client.head_bucket(Bucket=self.bucket)

        # Single PUT with user metadata, followed by HEAD and a full GET.
        single_payload = bytes(range(256)) * 2048
        metadata = {"test-client": "boto3", "test-purpose": "compatibility"}
        self.client.put_object(
            Bucket=self.bucket,
            Key=self.single_key,
            Body=single_payload,
            ContentType="application/octet-stream",
            Metadata=metadata,
        )
        head = self.client.head_object(Bucket=self.bucket, Key=self.single_key)
        self.assertEqual(head["ContentLength"], len(single_payload))
        self.assertEqual(head["ContentType"], "application/octet-stream")
        self.assertEqual(
            {key.lower(): value for key, value in head["Metadata"].items()},
            metadata,
        )

        full = self.client.get_object(Bucket=self.bucket, Key=self.single_key)
        with full["Body"]:
            self.assertEqual(full["Body"].read(), single_payload)

        # Range reads must return the requested plaintext span, not ciphertext
        # offsets from ARMOR's encrypted backing object.
        start, end = 37, 8192
        ranged = self.client.get_object(
            Bucket=self.bucket,
            Key=self.single_key,
            Range=f"bytes={start}-{end}",
        )
        with ranged["Body"]:
            self.assertEqual(ranged["ResponseMetadata"]["HTTPStatusCode"], 206)
            self.assertEqual(ranged["ContentRange"], f"bytes {start}-{end}/{len(single_payload)}")
            self.assertEqual(ranged["Body"].read(), single_payload[start : end + 1])

        # list_objects_v2 is the object-side bucket operation used by standard
        # boto3 callers and transfer managers.
        listed = self.client.list_objects_v2(Bucket=self.bucket, Prefix=self.prefix)
        self.assertIn(self.single_key, {entry["Key"] for entry in listed.get("Contents", [])})

        # Force boto3's managed multipart path with a valid non-final part and
        # a short final part. The transfer manager performs SigV4-signed
        # CreateMultipartUpload, concurrent UploadPart, and CompleteMultipartUpload
        # requests, which is the client behavior documented by ARMOR.
        multipart_payload = (b"part-one-" * (6 * 1024 * 1024 // 9)) + b"final-part"
        self.client.upload_fileobj(
            io.BytesIO(multipart_payload),
            self.bucket,
            self.multipart_key,
            ExtraArgs={"Metadata": {"test-client": "boto3-multipart"}},
            Config=TransferConfig(
                multipart_threshold=5 * 1024 * 1024,
                multipart_chunksize=5 * 1024 * 1024,
                max_concurrency=2,
                use_threads=True,
            ),
        )
        multipart_head = self.client.head_object(
            Bucket=self.bucket, Key=self.multipart_key
        )
        normalized_multipart_metadata = {
            key.lower(): value for key, value in multipart_head["Metadata"].items()
        }
        self.assertEqual(
            normalized_multipart_metadata["test-client"], "boto3-multipart"
        )
        multipart_get = self.client.get_object(
            Bucket=self.bucket, Key=self.multipart_key
        )
        with multipart_get["Body"]:
            self.assertEqual(multipart_get["Body"].read(), multipart_payload)

    def test_overwrite_replaces_object(self):
        """A later PUT replaces both the old bytes and metadata."""
        first_payload = b"first-version-" * 1000
        self.client.put_object(
            Bucket=self.bucket,
            Key=self.overwrite_key,
            Body=first_payload,
            Metadata={"revision": "1"},
        )

        second_payload = bytes(range(256)) * 33
        second_metadata = {"revision": "2"}
        self.client.put_object(
            Bucket=self.bucket,
            Key=self.overwrite_key,
            Body=second_payload,
            Metadata=second_metadata,
        )

        head = self.client.head_object(Bucket=self.bucket, Key=self.overwrite_key)
        self.assertEqual(head["ContentLength"], len(second_payload))
        self.assertEqual(
            {key.lower(): value for key, value in head["Metadata"].items()},
            second_metadata,
        )
        overwritten = self.client.get_object(
            Bucket=self.bucket, Key=self.overwrite_key
        )
        with overwritten["Body"]:
            self.assertEqual(overwritten["Body"].read(), second_payload)


if __name__ == "__main__":
    unittest.main()
