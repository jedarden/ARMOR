#!/usr/bin/env python3
"""
Example Test Table Extensions for ARMOR

This file provides comprehensive examples of how to extend the test table structure
for different error types and scenarios. Each example demonstrates a different pattern
for creating custom test tables.

Bead: bf-1a04kj
Created: 2026-07-15
"""

from dataclasses import dataclass
from typing import Dict, Any, List, Optional, Callable
from tests.test_tables import (
    TestCase,
    ErrorTestTable,
    TestResult,
    TestExecutionResult,
    TestTableResult,
    run_test_case,
    run_test_table
)


# =============================================================================
# EXAMPLE 1: S3-SPECIFIC ERROR TABLE
# =============================================================================

def create_s3_error_test_table() -> ErrorTestTable:
    """
    Create a test table for S3-specific error scenarios.

    This demonstrates how to create a test table for a specific protocol/service
    with its own error codes and response formats.

    Returns:
        ErrorTestTable: S3 error test table
    """
    return ErrorTestTable(
        name="s3_errors",
        description="S3 protocol-specific error scenarios",
        tags=["s3", "storage", "protocol"],
        test_cases=[
            # Bucket not found
            TestCase(
                id="S3-BUCKET-NF-001",
                description="Accessing non-existent S3 bucket",
                input_data={
                    "operation": "ListObjects",
                    "bucket": "nonexistent-bucket-12345"
                },
                expected_status=404,
                expected_error="NoSuchBucket",
                expected_message="The specified bucket does not exist",
                expected_fields={
                    "s3_error_code": "NoSuchBucket",
                    "resource": "/nonexistent-bucket-12345"
                },
                tags=["bucket", "not-found", "smoke"]
            ),
            # Object not found
            TestCase(
                id="S3-OBJ-NF-001",
                description="Accessing non-existent S3 object",
                input_data={
                    "operation": "GetObject",
                    "bucket": "test-bucket",
                    "key": "nonexistent-object.txt"
                },
                expected_status=404,
                expected_error="NoSuchKey",
                expected_message="The specified key does not exist",
                expected_fields={
                    "s3_error_code": "NoSuchKey",
                    "key": "nonexistent-object.txt"
                },
                tags=["object", "not-found", "smoke"]
            ),
            # Bucket already exists
            TestCase(
                id="S3-BUCKET-EXIST-001",
                description="Creating bucket that already exists",
                input_data={
                    "operation": "CreateBucket",
                    "bucket": "existing-bucket",
                    "region": "us-east-1"
                },
                expected_status=409,
                expected_error="BucketAlreadyExists",
                expected_message="bucket already exists",
                expected_fields={
                    "s3_error_code": "BucketAlreadyExists",
                    "bucket": "existing-bucket"
                },
                tags=["bucket", "conflict"]
            ),
            # Access denied
            TestCase(
                id="S3-DENIED-001",
                description="Access denied to private bucket",
                input_data={
                    "operation": "ListObjects",
                    "bucket": "private-bucket",
                    "credentials": "unauthorized-user"
                },
                expected_status=403,
                expected_error="AccessDenied",
                expected_message="Access Denied",
                expected_fields={
                    "s3_error_code": "AccessDenied",
                    "resource": "/private-bucket"
                },
                tags=["auth", "permissions", "critical"]
            ),
            # Invalid bucket name
            TestCase(
                id="S3-INVAL-BUCKET-001",
                description="Creating bucket with invalid name",
                input_data={
                    "operation": "CreateBucket",
                    "bucket": "Invalid_Bucket_Name_123"
                },
                expected_status=400,
                expected_error="InvalidBucketName",
                expected_message="bucket name does not conform",
                expected_fields={
                    "s3_error_code": "InvalidBucketName"
                },
                tags=["validation", "bucket-name"]
            ),
            # Invalid object key
            TestCase(
                id="S3-INVAL-KEY-001",
                description="Upload with invalid object key",
                input_data={
                    "operation": "PutObject",
                    "bucket": "test-bucket",
                    "key": "invalid//key/path"
                },
                expected_status=400,
                expected_error="InvalidObjectName",
                expected_message="object key contains invalid characters",
                tags=["validation", "object-key"]
            ),
            # Missing required header
            TestCase(
                id="S3-MISS-HDR-001",
                description="Upload without Content-Length header",
                input_data={
                    "operation": "PutObject",
                    "bucket": "test-bucket",
                    "key": "test.txt",
                    "headers": {"Content-Type": "text/plain"},
                    "missing_header": "Content-Length"
                },
                expected_status=411,
                expected_error="MissingContentLength",
                expected_message="Content-Length is required",
                tags=["validation", "headers", "smoke"]
            ),
            # Success case
            TestCase(
                id="S3-SUCCESS-001",
                description="Successful S3 object retrieval",
                input_data={
                    "operation": "GetObject",
                    "bucket": "test-bucket",
                    "key": "existing-object.txt",
                    "credentials": "authorized-user"
                },
                expected_status=200,
                expected_error=None,
                expected_fields={
                    "content_type": "text/plain",
                    "content_length": 1024
                },
                tags=["happy-path"]
            )
        ]
    )


# =============================================================================
# EXAMPLE 2: DATABASE ERROR TABLE
# =============================================================================

def create_database_error_test_table() -> ErrorTestTable:
    """
    Create a test table for database-related error scenarios.

    This demonstrates testing database-specific errors like connection issues,
    constraint violations, and transaction failures.

    Returns:
        ErrorTestTable: Database error test table
    """
    return ErrorTestTable(
        name="database_errors",
        description="Database operation error scenarios",
        tags=["database", "persistence", "backend"],
        test_cases=[
            # Connection timeout
            TestCase(
                id="DB-TIMEOUT-001",
                description="Database connection timeout",
                input_data={
                    "operation": "query",
                    "query": "SELECT * FROM users",
                    "timeout": 0.001  # Very short timeout
                },
                expected_status=503,
                expected_error="database_timeout",
                expected_message="Database connection timeout",
                expected_fields={
                    "error_type": "timeout",
                    "retry_after": 5
                },
                tags=["timeout", "critical"]
            ),
            # Connection refused
            TestCase(
                id="DB-CONN-001",
                description="Database connection refused",
                input_data={
                    "operation": "connect",
                    "host": "invalid-host",
                    "port": 5432
                },
                expected_status=503,
                expected_error="database_unavailable",
                expected_message="Connection refused",
                tags=["connection", "critical"]
            ),
            # Unique constraint violation
            TestCase(
                id="DB-CONST-001",
                description="Unique constraint violation",
                input_data={
                    "operation": "insert",
                    "table": "users",
                    "data": {"email": "existing@example.com"}  # Duplicate email
                },
                expected_status=409,
                expected_error="constraint_violation",
                expected_message="unique constraint",
                expected_fields={
                    "constraint_type": "unique",
                    "column": "email"
                },
                tags=["constraint", "conflict"]
            ),
            # Foreign key violation
            TestCase(
                id="DB-FK-001",
                description="Foreign key constraint violation",
                input_data={
                    "operation": "insert",
                    "table": "orders",
                    "data": {"user_id": 999999}  # Non-existent user
                },
                expected_status=400,
                expected_error="constraint_violation",
                expected_message="foreign key constraint",
                expected_fields={
                    "constraint_type": "foreign_key",
                    "column": "user_id"
                },
                tags=["constraint", "foreign-key"]
            ),
            # Null constraint violation
            TestCase(
                id="DB-NULL-001",
                description="NOT NULL constraint violation",
                input_data={
                    "operation": "insert",
                    "table": "users",
                    "data": {"email": None}  # Required field
                },
                expected_status=400,
                expected_error="constraint_violation",
                expected_message="NOT NULL constraint",
                expected_fields={
                    "constraint_type": "not_null",
                    "column": "email"
                },
                tags=["constraint", "not-null"]
            ),
            # Query syntax error
            TestCase(
                id="DB-SYNTAX-001",
                description="SQL syntax error",
                input_data={
                    "operation": "query",
                    "query": "SELECT FROM users"  # Invalid SQL
                },
                expected_status=400,
                expected_error="syntax_error",
                expected_message="syntax error",
                tags=["syntax", "validation"]
            ),
            # Transaction deadlock
            TestCase(
                id="DB-DEADLOCK-001",
                description="Database transaction deadlock",
                input_data={
                    "operation": "transaction",
                    "actions": ["lock_table_users", "lock_table_orders"],
                    "simulate_deadlock": True
                },
                expected_status=409,
                expected_error="deadlock",
                expected_message="deadlock detected",
                expected_fields={
                    "error_code": "deadlock",
                    "retry_possible": True
                },
                tags=["transaction", "deadlock"]
            ),
            # Success case
            TestCase(
                id="DB-SUCCESS-001",
                description="Successful database query",
                input_data={
                    "operation": "query",
                    "query": "SELECT * FROM users WHERE id = 1"
                },
                expected_status=200,
                expected_error=None,
                expected_fields={
                    "row_count": 1,
                    "execution_time_ms": 50
                },
                tags=["happy-path"]
            )
        ]
    )


# =============================================================================
# EXAMPLE 3: API RATE LIMITING TABLE
# =============================================================================

def create_rate_limiting_test_table() -> ErrorTestTable:
    """
    Create a comprehensive test table for rate limiting scenarios.

    This demonstrates testing various rate limiting strategies including
    token buckets, sliding windows, and fixed windows.

    Returns:
        ErrorTestTable: Rate limiting test table
    """
    return ErrorTestTable(
        name="rate_limiting",
        description="Rate limiting and throttling scenarios",
        tags=["rate_limit", "throttling", "api"],
        test_cases=[
            # Token bucket - limit exceeded
            TestCase(
                id="RL-TOKEN-001",
                description="Token bucket rate limit exceeded",
                input_data={
                    "endpoint": "/api/data",
                    "rate_limit_strategy": "token_bucket",
                    "capacity": 100,
                    "requests_made": 101
                },
                expected_status=429,
                expected_error="rate_limited",
                expected_message="Rate limit exceeded",
                expected_fields={
                    "strategy": "token_bucket",
                    "retry_after": 60,
                    "limit": 100
                },
                tags=["token-bucket", "smoke"]
            ),
            # Sliding window - limit exceeded
            TestCase(
                id="RL-SLIDING-001",
                description="Sliding window rate limit exceeded",
                input_data={
                    "endpoint": "/api/search",
                    "rate_limit_strategy": "sliding_window",
                    "window_seconds": 60,
                    "max_requests": 1000,
                    "requests_made": 1001
                },
                expected_status=429,
                expected_error="rate_limited",
                expected_message="Too many requests",
                expected_fields={
                    "strategy": "sliding_window",
                    "retry_after": 30,
                    "window": "60s"
                },
                tags=["sliding-window"]
            ),
            # Fixed window - limit exceeded
            TestCase(
                id="RL-FIXED-001",
                description="Fixed window rate limit exceeded",
                input_data={
                    "endpoint": "/api/updates",
                    "rate_limit_strategy": "fixed_window",
                    "window_minutes": 1,
                    "max_requests": 100,
                    "requests_made": 101
                },
                expected_status=429,
                expected_error="rate_limited",
                expected_message="Rate limit exceeded",
                expected_fields={
                    "strategy": "fixed_window",
                    "retry_after": 60,
                    "window_reset": "2026-07-15T12:01:00Z"
                },
                tags=["fixed-window"]
            ),
            # Per-user rate limit
            TestCase(
                id="RL-USER-001",
                description="Per-user rate limit exceeded",
                input_data={
                    "endpoint": "/api/user/data",
                    "user_id": "user-123",
                    "rate_limit_type": "per_user",
                    "limit_per_user": 100,
                    "requests_made": 101
                },
                expected_status=429,
                expected_error="rate_limited",
                expected_message="User rate limit exceeded",
                expected_fields={
                    "limit_type": "per_user",
                    "user_limit": 100,
                    "retry_after": 300
                },
                tags=["per-user"]
            ),
            # Per-IP rate limit
            TestCase(
                id="RL-IP-001",
                description="Per-IP rate limit exceeded",
                input_data={
                    "endpoint": "/api/public/data",
                    "client_ip": "192.168.1.100",
                    "rate_limit_type": "per_ip",
                    "limit_per_ip": 1000,
                    "requests_made": 1001
                },
                expected_status=429,
                expected_error="rate_limited",
                expected_message="IP rate limit exceeded",
                expected_fields={
                    "limit_type": "per_ip",
                    "ip_limit": 1000,
                    "retry_after": 3600
                },
                tags=["per-ip"]
            ),
            # Gradual recovery
            TestCase(
                id="RL-RECOVERY-001",
                description="Gradual recovery after rate limit",
                input_data={
                    "endpoint": "/api/data",
                    "rate_limit_strategy": "token_bucket",
                    "capacity": 100,
                    "requests_made": 100,
                    "wait_seconds": 10,
                    "subsequent_requests": 1
                },
                expected_status=200,
                expected_error=None,
                expected_fields={
                    "tokens_available": 10
                },
                tags=["recovery", "token-bucket"]
            ),
            # Burst capacity
            TestCase(
                id="RL-BURST-001",
                description="Burst capacity available",
                input_data={
                    "endpoint": "/api/data",
                    "rate_limit_strategy": "token_bucket",
                    "capacity": 100,
                    "burst_requests": 50,
                    "duration_seconds": 1
                },
                expected_status=200,
                expected_error=None,
                expected_fields={
                    "burst_capacity": 50,
                    "tokens_remaining": 50
                },
                tags=["burst", "happy-path"]
            ),
            # Success case
            TestCase(
                id="RL-SUCCESS-001",
                description="Request within rate limit",
                input_data={
                    "endpoint": "/api/data",
                    "rate_limit_strategy": "token_bucket",
                    "capacity": 100,
                    "requests_made": 50
                },
                expected_status=200,
                expected_error=None,
                expected_fields={
                    "tokens_remaining": 50,
                    "limit": 100
                },
                tags=["happy-path", "smoke"]
            )
        ]
    )


# =============================================================================
# EXAMPLE 4: NETWORK ERROR TABLE
# =============================================================================

def create_network_error_test_table() -> ErrorTestTable:
    """
    Create a test table for network-related error scenarios.

    This demonstrates testing network errors like timeouts, connection
    failures, and DNS resolution issues.

    Returns:
        ErrorTestTable: Network error test table
    """
    return ErrorTestTable(
        name="network_errors",
        description="Network-related error scenarios",
        tags=["network", "connectivity", "infrastructure"],
        test_cases=[
            # Connection timeout
            TestCase(
                id="NET-TIMEOUT-001",
                description="Connection timeout to external service",
                input_data={
                    "operation": "http_request",
                    "url": "http://slow-service.example.com/api",
                    "timeout_seconds": 5,
                    "simulate": "timeout"
                },
                expected_status=504,
                expected_error="gateway_timeout",
                expected_message="Connection timeout",
                expected_fields={
                    "timeout_seconds": 5,
                    "remote_service": "slow-service.example.com"
                },
                tags=["timeout", "critical"]
            ),
            # Connection refused
            TestCase(
                id="NET-REFUSED-001",
                description="Connection refused by remote host",
                input_data={
                    "operation": "http_request",
                    "url": "http://localhost:9999/api",
                    "simulate": "connection_refused"
                },
                expected_status=503,
                expected_error="service_unavailable",
                expected_message="Connection refused",
                tags=["connection", "critical"]
            ),
            # DNS resolution failure
            TestCase(
                id="NET-DNS-001",
                description="DNS resolution failure",
                input_data={
                    "operation": "http_request",
                    "url": "http://nonexistent-domain.invalid/api"
                },
                expected_status=502,
                expected_error="bad_gateway",
                expected_message="DNS resolution failed",
                expected_fields={
                    "error_type": "dns_failure",
                    "hostname": "nonexistent-domain.invalid"
                },
                tags=["dns", "resolution"]
            ),
            # SSL/TLS error
            TestCase(
                id="NET-SSL-001",
                description="SSL/TLS certificate error",
                input_data={
                    "operation": "https_request",
                    "url": "https://expired-cert.example.com/api",
                    "simulate": "ssl_error"
                },
                expected_status=502,
                expected_error="bad_gateway",
                expected_message="SSL certificate error",
                expected_fields={
                    "error_type": "ssl_error",
                    "certificate": "expired"
                },
                tags=["ssl", "tls", "security"]
            ),
            # Network unreachable
            TestCase(
                id="NET-UNREACH-001",
                description="Network unreachable",
                input_data={
                    "operation": "http_request",
                    "url": "http://10.255.255.1/api",
                    "simulate": "network_unreachable"
                },
                expected_status=503,
                expected_error="service_unavailable",
                expected_message="Network unreachable",
                tags=["network", "routing"]
            ),
            # Connection reset
            TestCase(
                id="NET-RESET-001",
                description="Connection reset by peer",
                input_data={
                    "operation": "http_request",
                    "url": "http://flaky-service.example.com/api",
                    "simulate": "connection_reset"
                },
                expected_status=502,
                expected_error="bad_gateway",
                expected_message="Connection reset",
                tags=["connection", "reset"]
            ),
            # Success case
            TestCase(
                id="NET-SUCCESS-001",
                description="Successful network request",
                input_data={
                    "operation": "http_request",
                    "url": "http://stable-service.example.com/api/health"
                },
                expected_status=200,
                expected_error=None,
                expected_fields={
                    "response_time_ms": 50,
                    "content_type": "application/json"
                },
                tags=["happy-path"]
            )
        ]
    )


# =============================================================================
# EXAMPLE 5: FILESYSTEM ERROR TABLE
# =============================================================================

def create_filesystem_error_test_table() -> ErrorTestTable:
    """
    Create a test table for filesystem-related error scenarios.

    This demonstrates testing filesystem errors like permission issues,
    disk space problems, and file not found errors.

    Returns:
        ErrorTestTable: Filesystem error test table
    """
    return ErrorTestTable(
        name="filesystem_errors",
        description="Filesystem operation error scenarios",
        tags=["filesystem", "storage", "io"],
        test_cases=[
            # File not found
            TestCase(
                id="FS-NF-001",
                description="File not found",
                input_data={
                    "operation": "read_file",
                    "path": "/nonexistent/file.txt"
                },
                expected_status=404,
                expected_error="file_not_found",
                expected_message="File not found",
                expected_fields={
                    "path": "/nonexistent/file.txt"
                },
                tags=["not-found", "smoke"]
            ),
            # Permission denied
            TestCase(
                id="FS-PERM-001",
                description="Permission denied",
                input_data={
                    "operation": "write_file",
                    "path": "/root/protected.txt",
                    "user": "unprivileged_user"
                },
                expected_status=403,
                expected_error="permission_denied",
                expected_message="Permission denied",
                expected_fields={
                    "required_permission": "write",
                    "path": "/root/protected.txt"
                },
                tags=["permissions", "security"]
            ),
            # Disk full
            TestCase(
                id="FS-FULL-001",
                description="Disk full",
                input_data={
                    "operation": "write_file",
                    "path": "/data/large_file.dat",
                    "size_mb": 10240,
                    "simulate": "disk_full"
                },
                expected_status=507,
                expected_error="insufficient_storage",
                expected_message="No space left on device",
                expected_fields={
                    "available_space": 0,
                    "required_space": 10737418240
                },
                tags=["storage", "critical"]
            ),
            # Directory not found
            TestCase(
                id="FS-DIR-NF-001",
                description="Directory not found",
                input_data={
                    "operation": "write_file",
                    "path": "/nonexistent_dir/file.txt"
                },
                expected_status=404,
                expected_error="directory_not_found",
                expected_message="Directory not found",
                expected_fields={
                    "path": "/nonexistent_dir"
                },
                tags=["directory", "not-found"]
            ),
            # File too large
            TestCase(
                id="FS-SIZE-001",
                description="File size exceeds limit",
                input_data={
                    "operation": "upload_file",
                    "file_size": 10737418240,  # 10GB
                    "max_size": 104857600  # 100MB
                },
                expected_status=413,
                expected_error="file_too_large",
                expected_message="File too large",
                expected_fields={
                    "file_size": 10737418240,
                    "max_size": 104857600
                },
                tags=["validation", "size"]
            ),
            # Is a directory
            TestCase(
                id="FS-ISDIR-001",
                description="Path is a directory",
                input_data={
                    "operation": "read_file",
                    "path": "/var/log"  # This is a directory
                },
                expected_status=400,
                expected_error="is_directory",
                expected_message="Is a directory",
                expected_fields={
                    "path": "/var/log",
                    "expected_type": "file"
                },
                tags=["validation", "type"]
            ),
            # Success case
            TestCase(
                id="FS-SUCCESS-001",
                description="Successful file read",
                input_data={
                    "operation": "read_file",
                    "path": "/etc/hostname"
                },
                expected_status=200,
                expected_error=None,
                expected_fields={
                    "size": 12,
                    "content_type": "text/plain"
                },
                tags=["happy-path"]
            )
        ]
    )


# =============================================================================
# EXAMPLE 6: BUSINESS LOGIC ERROR TABLE
# =============================================================================

def create_business_logic_error_test_table() -> ErrorTestTable:
    """
    Create a test table for business logic error scenarios.

    This demonstrates testing domain-specific business rules and logic errors.

    Returns:
        ErrorTestTable: Business logic error test table
    """
    return ErrorTestTable(
        name="business_logic_errors",
        description="Business logic validation error scenarios",
        tags=["business", "domain", "validation"],
        test_cases=[
            # Insufficient funds
            TestCase(
                id="BL-FUNDS-001",
                description="Insufficient funds for transaction",
                input_data={
                    "operation": "transfer",
                    "from_account": "ACC-001",
                    "to_account": "ACC-002",
                    "amount": 10000,
                    "balance": 5000
                },
                expected_status=400,
                expected_error="insufficient_funds",
                expected_message="Insufficient funds",
                expected_fields={
                    "available_balance": 5000,
                    "required_amount": 10000,
                    "shortfall": 5000
                },
                tags=["financial", "funds", "smoke"]
            ),
            # Invalid state transition
            TestCase(
                id="BL-STATE-001",
                description="Invalid state transition",
                input_data={
                    "operation": "update_order_status",
                    "order_id": "ORD-001",
                    "current_status": "cancelled",
                    "new_status": "shipped"
                },
                expected_status=400,
                expected_error="invalid_state_transition",
                expected_message="Cannot transition from cancelled to shipped",
                expected_fields={
                    "current_state": "cancelled",
                    "requested_state": "shipped",
                    "valid_transitions": ["pending", "processing"]
                },
                tags=["state-machine", "workflow"]
            ),
            # Duplicate resource
            TestCase(
                id="BL-DUP-001",
                description="Duplicate resource creation",
                input_data={
                    "operation": "create_user",
                    "email": "existing@example.com",
                    "check_duplicates": True
                },
                expected_status=409,
                expected_error="resource_exists",
                expected_message="User already exists",
                expected_fields={
                    "field": "email",
                    "value": "existing@example.com"
                },
                tags=["duplicate", "conflict"]
            ),
            # Business rule violation
            TestCase(
                id="BL-RULE-001",
                description="Business rule violation",
                input_data={
                    "operation": "create_order",
                    "items": [{"product_id": "PROD-001", "quantity": 1000}],
                    "max_quantity_per_order": 100
                },
                expected_status=400,
                expected_error="business_rule_violation",
                expected_message="Maximum quantity per order is 100",
                expected_fields={
                    "rule": "max_quantity_per_order",
                    "limit": 100,
                    "provided": 1000
                },
                tags=["business-rule", "validation"]
            ),
            # Concurrent modification
            TestCase(
                id="BL-CONCURRENT-001",
                description="Concurrent modification conflict",
                input_data={
                    "operation": "update_resource",
                    "resource_id": "RES-001",
                    "version": 1,
                    "current_version": 2
                },
                expected_status=409,
                expected_error="concurrent_modification",
                expected_message="Resource has been modified by another user",
                expected_fields={
                    "your_version": 1,
                    "current_version": 2
                },
                tags=["concurrency", "optimistic-locking"]
            ),
            # Dependency not satisfied
            TestCase(
                id="BL-DEP-001",
                description="Required dependency not satisfied",
                input_data={
                    "operation": "deploy_service",
                    "service": "payment-processor",
                    "dependencies": ["database", "cache"],
                    "available_dependencies": ["database"]
                },
                expected_status=400,
                expected_error="dependency_not_satisfied",
                expected_message="Required dependency not available",
                expected_fields={
                    "missing_dependency": "cache",
                    "service": "payment-processor"
                },
                tags=["dependency", "deployment"]
            ),
            # Success case
            TestCase(
                id="BL-SUCCESS-001",
                description="Valid business operation",
                input_data={
                    "operation": "transfer",
                    "from_account": "ACC-001",
                    "to_account": "ACC-002",
                    "amount": 100,
                    "balance": 1000
                },
                expected_status=200,
                expected_error=None,
                expected_fields={
                    "transaction_id": "TXN-001",
                    "new_balance": 900
                },
                tags=["happy-path"]
            )
        ]
    )


# =============================================================================
# HELPER FUNCTIONS
# =============================================================================

def register_all_extension_tables() -> Dict[str, ErrorTestTable]:
    """
    Register all extension test tables.

    Returns:
        Dict[str, ErrorTestTable]: Dictionary of all extension tables
    """
    return {
        's3_errors': create_s3_error_test_table(),
        'database_errors': create_database_error_test_table(),
        'rate_limiting': create_rate_limiting_test_table(),
        'network_errors': create_network_error_test_table(),
        'filesystem_errors': create_filesystem_error_test_table(),
        'business_logic_errors': create_business_logic_error_test_table()
    }


def get_extension_table(name: str) -> Optional[ErrorTestTable]:
    """
    Get an extension test table by name.

    Args:
        name: Table name

    Returns:
        ErrorTestTable or None
    """
    tables = register_all_extension_tables()
    return tables.get(name)


def list_extension_tables() -> List[str]:
    """
    List all available extension test tables.

    Returns:
        List[str]: Sorted list of table names
    """
    tables = register_all_extension_tables()
    return sorted(tables.keys())


# =============================================================================
# DEMO EXECUTORS
# =============================================================================

def demo_s3_executor(input_data: Dict[str, Any]) -> Dict[str, Any]:
    """
    Demo executor for S3 test cases.

    In a real implementation, this would make actual S3 API calls.
    For demo purposes, it returns mock responses.
    """
    operation = input_data.get("operation")
    bucket = input_data.get("bucket")

    # Mock responses based on input
    if operation == "CreateBucket" and bucket == "existing-bucket":
        return {
            'status': 409,
            'error': 'BucketAlreadyExists',
            'message': 'bucket already exists',
            'fields': {
                's3_error_code': 'BucketAlreadyExists',
                'bucket': bucket
            }
        }
    elif operation == "CreateBucket" and "Invalid" in bucket:
        return {
            'status': 400,
            'error': 'InvalidBucketName',
            'message': 'bucket name does not conform',
            'fields': {
                's3_error_code': 'InvalidBucketName'
            }
        }
    elif bucket == "nonexistent-bucket-12345":
        return {
            'status': 404,
            'error': 'NoSuchBucket',
            'message': 'The specified bucket does not exist',
            'fields': {
                's3_error_code': 'NoSuchBucket',
                'resource': f'/{bucket}'
            }
        }
    else:
        return {
            'status': 200,
            'error': None,
            'message': 'OK',
            'fields': {
                'content_type': 'text/plain',
                'content_length': 1024
            }
        }


def demo_database_executor(input_data: Dict[str, Any]) -> Dict[str, Any]:
    """
    Demo executor for database test cases.
    """
    operation = input_data.get("operation")

    if operation == "connect" and input_data.get("host") == "invalid-host":
        return {
            'status': 503,
            'error': 'database_unavailable',
            'message': 'Connection refused',
            'fields': {}
        }
    elif operation == "insert" and "constraint" in str(input_data):
        return {
            'status': 409,
            'error': 'constraint_violation',
            'message': 'unique constraint',
            'fields': {
                'constraint_type': 'unique',
                'column': 'email'
            }
        }
    elif operation == "query":
        return {
            'status': 200,
            'error': None,
            'message': 'OK',
            'fields': {
                'row_count': 1,
                'execution_time_ms': 50
            }
        }
    else:
        return {
            'status': 200,
            'error': None,
            'message': 'OK',
            'fields': {}
        }


# =============================================================================
# MAIN DEMO
# =============================================================================

def main():
    """Demonstrate usage of extension test tables."""
    import sys

    print("ARMOR Test Table Extensions Demo")
    print("=" * 50)
    print()

    # List available tables
    print("Available Extension Tables:")
    for table_name in list_extension_tables():
        table = get_extension_table(table_name)
        print(f"  - {table_name}: {len(table.test_cases)} test cases")
    print()

    # Run S3 error tests
    print("Running S3 Error Tests...")
    s3_table = get_extension_table('s3_errors')
    s3_results = run_test_table(s3_table, demo_s3_executor)
    print(f"  Results: {s3_results.passed_count}/{s3_results.total_count} passed")
    print()

    # Run database error tests
    print("Running Database Error Tests...")
    db_table = get_extension_table('database_errors')
    db_results = run_test_table(db_table, demo_database_executor)
    print(f"  Results: {db_results.passed_count}/{db_results.total_count} passed")
    print()

    return 0


if __name__ == '__main__':
    import sys
    sys.exit(main())
