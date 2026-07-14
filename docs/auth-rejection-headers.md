# ARMOR Authentication Rejection Response Headers

This document documents the exact response headers returned for all authentication rejection scenarios.

## Overview

All authentication rejection scenarios return consistent response headers with an HTTP status code of 403 Forbidden and an XML-formatted error response body.

## Common Response Headers (All Scenarios)

All authentication rejection scenarios return the following headers:

```
Content-Type: application/xml
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length
```

**Note:** Header order may vary between responses, but the same set of headers is always present.

## Authentication Rejection Scenarios

### MissingAuthenticationToken

**Trigger:** Authorization header is missing

**HTTP Status Code:** `403`

**Response Headers:**
```
Content-Type: application/xml
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length
```

**Response Body:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MissingAuthenticationToken</Code>
  <Message>Missing Authentication Token</Message>
</Error>
```

**Error Details:**
- **Error Code:** `MissingAuthenticationToken`
- **Error Message:** Missing Authentication Token

---

### InvalidAccessKeyId

**Trigger:** The provided access key does not exist

**HTTP Status Code:** `403`

**Response Headers:**
```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length
Content-Type: application/xml
```

**Response Body:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidAccessKeyId</Code>
  <Message>The AWS Access Key Id you provided does not exist</Message>
</Error>
```

**Error Details:**
- **Error Code:** `InvalidAccessKeyId`
- **Error Message:** The AWS Access Key Id you provided does not exist

---

### SignatureDoesNotMatch

**Trigger:** Calculated signature does not match provided signature

**HTTP Status Code:** `403`

**Response Headers:**
```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length
Content-Type: application/xml
```

**Response Body:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>SignatureDoesNotMatch</Code>
  <Message>The request signature we calculated does not match the signature you provided</Message>
</Error>
```

**Error Details:**
- **Error Code:** `SignatureDoesNotMatch`
- **Error Message:** The request signature we calculated does not match the signature you provided

---

### InvalidAlgorithm

**Trigger:** Authorization header does not use AWS4-HMAC-SHA256

**HTTP Status Code:** `403`

**Response Headers:**
```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length
Content-Type: application/xml
```

**Response Body:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidAlgorithm</Code>
  <Message>Only AWS4-HMAC-SHA256 is supported</Message>
</Error>
```

**Error Details:**
- **Error Code:** `InvalidAlgorithm`
- **Error Message:** Only AWS4-HMAC-SHA256 is supported

---

### MissingDateHeader

**Trigger:** X-Amz-Date header is missing

**HTTP Status Code:** `403`

**Response Headers:**
```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length
Content-Type: application/xml
```

**Response Body:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MissingDateHeader</Code>
  <Message>Missing X-Amz-Date header</Message>
</Error>
```

**Error Details:**
- **Error Code:** `MissingDateHeader`
- **Error Message:** Missing X-Amz-Date header

---

### RequestExpired

**Trigger:** Request timestamp is outside allowed time window (15 minutes)

**HTTP Status Code:** `403`

**Response Headers:**
```
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length
Content-Type: application/xml
Access-Control-Allow-Origin: *
```

**Response Body:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>RequestExpired</Code>
  <Message>Request has expired</Message>
</Error>
```

**Error Details:**
- **Error Code:** `RequestExpired`
- **Error Message:** Request has expired

---

### IncompleteSignature

**Trigger:** Authorization header is missing required fields

**HTTP Status Code:** `403`

**Response Headers:**
```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length
Content-Type: application/xml
```

**Response Body:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>IncompleteSignature</Code>
  <Message>Authorization header is missing required fields</Message>
</Error>
```

**Error Details:**
- **Error Code:** `IncompleteSignature`
- **Error Message:** Authorization header is missing required fields

---

## Consistency Summary

### Header Consistency
✅ **All authentication rejection scenarios return the same set of headers:**
- `Content-Type: application/xml`
- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS`
- `Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length`

### Status Code Consistency
✅ **All authentication rejection scenarios return HTTP 403 Forbidden**

### Response Format Consistency
✅ **All authentication rejection scenarios return XML-formatted error responses:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>[Error Code]</Code>
  <Message>[Error Message]</Message>
</Error>
```

## Additional Authentication Error Codes

The following error codes are also supported but not covered in this documentation:

- **InvalidCredential** - Invalid credential format in Authorization header
- **InvalidDateFormat** - Invalid date format in X-Amz-Date header
- **AccessDenied** - ACL-based access control rejection (authorization, not authentication)

## Performance Characteristics

Based on comprehensive error verification testing (`internal/server/error_response_verification_test.go`):

- **Average response time:** <20µs for local testing
- **Maximum response time:** <100ms under normal conditions
- **Response time includes:** Authentication verification, signature calculation, and error response generation

## S3 Compatibility

These response headers and error codes follow the AWS S3 API specification for authentication errors, ensuring compatibility with S3 clients and tools.
