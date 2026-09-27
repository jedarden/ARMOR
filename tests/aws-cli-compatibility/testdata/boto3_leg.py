#!/usr/bin/env python3
"""Small in-suite boto3 leg: botocore SigV4 against the real ARMOR pipeline.

This is the sibling of ``tests/test_s3_basic_operations.py`` (the
built-image boto3 gate, which runs in endpoint mode only). This leg keeps the
suite's client matrix complete wherever ``go test`` runs: the Go test in
``boto3_compat_test.go`` shells out to it against either the in-process ARMOR
server or, in ARMOR_COMPAT_ENDPOINT mode, a running deployment.

Coverage follows the README promise for boto3: reads, range reads and
single-PUT writes with metadata — put/get byte equality, a bounded Range
request, head_object, list_objects_v2, delete with its subsequent NoSuchKey —
plus overwrite (a second PUT replaces the payload), an explicit multipart
upload (create_multipart_upload / upload_part / list_parts /
complete_multipart_upload with a short final part and a part-boundary range
read), and the abort path (no object materializes, the upload's parts become
unreachable).

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

MIB = 1024 * 1024


def fail(message):
    print("FAIL: %s" % message, file=sys.stderr)
    return 1


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
        return fail("get_object body differs from the uploaded payload")

    # Bounded range read.
    rng = client.get_object(Bucket=bucket, Key=key, Range="bytes=1000-1999")
    chunk = rng["Body"].read()
    if chunk != payload[1000:2000] or rng["ContentLength"] != 1000:
        return fail("Range read bytes=1000-1999 returned the wrong slice")

    # HEAD.
    head = client.head_object(Bucket=bucket, Key=key)
    if head["ContentLength"] != len(payload):
        return fail("head_object ContentLength mismatch")

    # Listing under the leg's prefix.
    listed = client.list_objects_v2(Bucket=bucket, Prefix=prefix + "/")
    keys = [o["Key"] for o in listed.get("Contents", [])]
    if key not in keys:
        return fail("list_objects_v2 did not list the uploaded key")

    # Delete, then confirm the read fails with NoSuchKey.
    client.delete_object(Bucket=bucket, Key=key)
    try:
        client.get_object(Bucket=bucket, Key=key)
    except client.exceptions.NoSuchKey:
        pass
    else:
        return fail("get_object after delete did not raise NoSuchKey")

    # Overwrite: a second PUT to the same key replaces the stored payload —
    # the read sees only the new bytes and HEAD reports the new length.
    overwrite_key = prefix + "/overwrite.bin"
    first = hashlib.sha256(b"armor-compat-overwrite-first").digest() * 8192
    second = hashlib.sha256(b"armor-compat-overwrite-second").digest() * 4096
    client.put_object(Bucket=bucket, Key=overwrite_key, Body=first)
    client.put_object(Bucket=bucket, Key=overwrite_key, Body=second)
    got = client.get_object(Bucket=bucket, Key=overwrite_key)
    if got["Body"].read() != second:
        return fail("read after overwrite did not return the replacement payload")
    if client.head_object(Bucket=bucket, Key=overwrite_key)["ContentLength"] != len(second):
        return fail("head_object after overwrite reported the stale length")

    # Explicit multipart: three parts with a short final one, assembled
    # server-side. The part-boundary range read proves the assembled object is
    # seekable, not just whole-file readable.
    mp_key = prefix + "/multipart.bin"
    part_a = hashlib.sha256(b"armor-compat-part-a").digest() * (5 * MIB // 32)
    part_b = hashlib.sha256(b"armor-compat-part-b").digest() * (5 * MIB // 32)
    part_c = hashlib.sha256(b"armor-compat-part-c").digest() * (MIB // 32)
    mp_payload = part_a + part_b + part_c
    boundary = len(part_a)

    created = client.create_multipart_upload(Bucket=bucket, Key=mp_key)
    upload_id = created["UploadId"]
    parts = []
    for number, chunk in ((1, part_a), (2, part_b), (3, part_c)):
        uploaded = client.upload_part(
            Bucket=bucket, Key=mp_key, UploadId=upload_id, PartNumber=number, Body=chunk
        )
        parts.append({"ETag": uploaded["ETag"], "PartNumber": number})

    listed = client.list_parts(Bucket=bucket, Key=mp_key, UploadId=upload_id)
    sizes = {p["PartNumber"]: p["Size"] for p in listed.get("Parts", [])}
    if sizes != {1: len(part_a), 2: len(part_b), 3: len(part_c)}:
        return fail("list_parts reported wrong parts/sizes: %r" % (sizes,))

    client.complete_multipart_upload(
        Bucket=bucket, Key=mp_key, UploadId=upload_id, MultipartUpload={"Parts": parts}
    )
    if client.head_object(Bucket=bucket, Key=mp_key)["ContentLength"] != len(mp_payload):
        return fail("completed multipart object has the wrong ContentLength")
    got = client.get_object(Bucket=bucket, Key=mp_key)
    if got["Body"].read() != mp_payload:
        return fail("completed multipart object does not round-trip byte-for-byte")
    rng = client.get_object(
        Bucket=bucket, Key=mp_key, Range="bytes=%d-%d" % (boundary - 10, boundary + 10)
    )
    if rng["Body"].read() != mp_payload[boundary - 10 : boundary + 11]:
        return fail("range read across the multipart part boundary returned the wrong slice")

    # Abort path: an aborted upload materializes no object, and its parts are
    # gone — the real deployment answers NoSuchUpload, the in-process harness
    # an empty part list, and both mean "nothing left to complete".
    abort_key = prefix + "/aborted.bin"
    created = client.create_multipart_upload(Bucket=bucket, Key=abort_key)
    abort_id = created["UploadId"]
    client.upload_part(
        Bucket=bucket, Key=abort_key, UploadId=abort_id, PartNumber=1, Body=part_a
    )
    client.abort_multipart_upload(Bucket=bucket, Key=abort_key, UploadId=abort_id)
    try:
        client.get_object(Bucket=bucket, Key=abort_key)
    except client.exceptions.NoSuchKey:
        pass
    else:
        return fail("get_object on an aborted upload's key did not raise NoSuchKey")
    try:
        leftover = client.list_parts(Bucket=bucket, Key=abort_key, UploadId=abort_id)
        if leftover.get("Parts"):
            return fail("parts of an aborted upload are still listed")
    except client.exceptions.NoSuchUpload:
        pass

    print(
        "boto3 leg OK: %s (%d bytes round-tripped, %d-byte multipart, overwrite and abort verified)"
        % (key, len(payload), len(mp_payload))
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
