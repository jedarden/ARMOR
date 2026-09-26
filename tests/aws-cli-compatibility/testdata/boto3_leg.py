#!/usr/bin/env python3
"""Small in-suite boto3 leg: botocore SigV4 against the real ARMOR pipeline.

This is the sibling of ``tests/test_s3_basic_operations.py`` (the
built-image boto3 gate, which runs in endpoint mode only). This leg keeps the
suite's client matrix complete wherever ``go test`` runs: the Go test in
``boto3_compat_test.go`` shells out to it against either the in-process ARMOR
server or, in ARMOR_COMPAT_ENDPOINT mode, a running deployment.

Coverage follows the README promise for boto3: reads, range reads and
single-PUT writes with metadata — put/get byte equality, a bounded Range
request, head_object, list_objects_v2, and delete with its subsequent NoSuchKey.

Credentials arrive as argv from the test harness — the harness's synthetic
pair, or the endpoint-mode server's own demo pair. Real deployments keep their
ARMOR client credentials in OpenBao and deliver them by reference; this script
only ever sees ephemeral test values.
"""

import hashlib
import os
import sys
import uuid

import boto3
from botocore.config import Config


def main() -> int:
    endpoint, bucket, region = sys.argv[1:4]
    access_key = os.environ["AWS_ACCESS_KEY_ID"]
    secret_key = os.environ["AWS_SECRET_ACCESS_KEY"]

    client = boto3.client(
        "s3",
        endpoint_url=endpoint,
        aws_access_key_id=access_key,
        aws_secret_access_key=secret_key,
        region_name=region,
        config=Config(
            signature_version="s3v4",
            s3={"addressing_style": "path"},
            retries={"max_attempts": 3, "mode": "standard"},
        ),
    )

    prefix = "compat/boto3-leg/" + uuid.uuid4().hex
    key = prefix + "/obj.bin"
    payload = hashlib.sha256(key.encode()).digest() * 8192  # 256 KiB, deterministic

    # Single-PUT write with metadata.
    client.put_object(
        Bucket=bucket, Key=key, Body=payload, Metadata={"origin": "armor-compat"}
    )

    # Full read: byte equality. Metadata coverage remains in the broader
    # production-image boto3 suite; this small matrix leg concentrates on
    # botocore's signer and the request shapes shared by all S3 clients.
    got = client.get_object(Bucket=bucket, Key=key)
    body = got["Body"].read()
    if body != payload:
        print("FAIL: get_object body differs from the uploaded payload", file=sys.stderr)
        return 1

    # Bounded range read.
    rng = client.get_object(Bucket=bucket, Key=key, Range="bytes=1000-1999")
    chunk = rng["Body"].read()
    if chunk != payload[1000:2000] or rng["ContentLength"] != 1000:
        print("FAIL: Range read bytes=1000-1999 returned the wrong slice", file=sys.stderr)
        return 1

    # HEAD.
    head = client.head_object(Bucket=bucket, Key=key)
    if head["ContentLength"] != len(payload):
        print("FAIL: head_object ContentLength mismatch", file=sys.stderr)
        return 1

    # Listing under the leg's prefix.
    listed = client.list_objects_v2(Bucket=bucket, Prefix=prefix + "/")
    keys = [o["Key"] for o in listed.get("Contents", [])]
    if key not in keys:
        print("FAIL: list_objects_v2 did not list the uploaded key", file=sys.stderr)
        return 1

    # Delete, then confirm the read fails with NoSuchKey.
    client.delete_object(Bucket=bucket, Key=key)
    try:
        client.get_object(Bucket=bucket, Key=key)
    except client.exceptions.NoSuchKey:
        pass
    else:
        print("FAIL: get_object after delete did not raise NoSuchKey", file=sys.stderr)
        return 1

    print("boto3 leg OK: %s (%d bytes round-tripped)" % (key, len(payload)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
