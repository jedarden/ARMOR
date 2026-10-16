// Package s3errors contains the single client-facing S3 error catalogue.
//
// Call sites may provide a useful diagnostic string, but that string is log
// detail, not a wire message. Keeping the two values separate prevents
// backend errors, implementation names, and operator remediation URLs from
// escaping to S3 clients.
package s3errors

import (
	"fmt"
	"strings"
)

// Request supplies the request context needed for dynamic catalogue entries.
type Request struct {
	Method    string
	Path      string
	RequestID string
	Status    int
}

// Entry is the resolved error record. LogDetail is intentionally the detail
// supplied by the handler; it is emitted only in the structured error log.
type Entry struct {
	ClientMessage string
	LogDetail     string
}

// Resolve maps an S3 error code to its client message while retaining the
// supplied diagnostic as structured-log detail.
func Resolve(code string, request Request, detail string) Entry {
	return Entry{
		ClientMessage: clientMessage(code, request, detail),
		LogDetail:     detail,
	}
}

func clientMessage(code string, request Request, detail string) string {
	switch code {
	case "AccessDenied":
		verb := strings.ToUpper(strings.TrimSpace(request.Method))
		if verb == "" {
			verb = "This request"
		} else {
			verb += " access"
		}
		path := request.Path
		if path == "" {
			path = "/"
		}
		return fmt.Sprintf("Access denied: %s to %s. Check the credential permissions for this operation.%s", verb, path, requestReference(request.RequestID))
	case "RequestExpired":
		return "The request has expired or its timestamp is outside the allowed 15-minute clock-skew window. Synchronize the client clock and retry." + requestReference(request.RequestID)
	case "InvalidRange":
		if strings.Contains(strings.ToLower(detail), "compress") {
			return "This object is compressed, so byte-range reads are unavailable. Retry without the Range header or upload the object without compression." + requestReference(request.RequestID)
		}
		return "The requested byte range is invalid. Use a range within the object length and retry." + requestReference(request.RequestID)
	case "InternalError":
		return "The server could not complete this request. Retry it and include the request ID if the failure continues." + requestReference(request.RequestID)
	case "RequestTimeout":
		return "The request exceeded the server deadline. Retry it." + requestReference(request.RequestID)
	case "ServiceUnavailable":
		return "The service is temporarily unavailable. Retry the request." + requestReference(request.RequestID)
	case "ClientDisconnect":
		return "The client disconnected before the response completed."
	case "MissingAuthenticationToken":
		return "Provide an AWS Signature V4 Authorization header and retry." + requestReference(request.RequestID)
	case "InvalidAccessKeyId":
		return "The AWS Access Key Id you provided does not exist. Check the access key and retry." + requestReference(request.RequestID)
	case "SignatureDoesNotMatch":
		return "The request signature does not match. Check the signing key, region, and signed headers, then retry." + requestReference(request.RequestID)
	case "InvalidAlgorithm":
		return "Use AWS Signature V4 authentication and retry." + requestReference(request.RequestID)
	case "IncompleteSignature":
		return "The Authorization header is missing required AWS Signature V4 fields. Correct it and retry." + requestReference(request.RequestID)
	case "InvalidCredential":
		return "The credential format is invalid. Correct the credential scope and retry." + requestReference(request.RequestID)
	case "MissingDateHeader":
		return "Provide the X-Amz-Date header and retry." + requestReference(request.RequestID)
	case "InvalidDateFormat":
		return "Format X-Amz-Date as an ISO8601 basic timestamp and retry." + requestReference(request.RequestID)
	case "AuthorizationQueryParametersError":
		return "X-Amz-Expires must be an integer between 1 and 604800 seconds. Correct it and retry." + requestReference(request.RequestID)
	case "NoSuchKey":
		return "The specified object does not exist." + requestReference(request.RequestID)
	case "NoSuchBucket":
		return "The specified bucket does not exist." + requestReference(request.RequestID)
	case "NoSuchUpload":
		return "The specified multipart upload does not exist. Check the upload ID and retry." + requestReference(request.RequestID)
	case "MalformedXML":
		return "The request body is not valid XML. Correct the XML and retry." + requestReference(request.RequestID)
	case "InvalidArgument", "InvalidRequest":
		return "The request contains an invalid argument. Correct the request and retry." + requestReference(request.RequestID)
	case "InvalidCopySource":
		return "The copy source is invalid. Provide a valid source object and retry." + requestReference(request.RequestID)
	case "MethodNotAllowed":
		return "The requested HTTP method is not supported for this resource. Use a supported S3 method and retry." + requestReference(request.RequestID)
	case "PreconditionFailed":
		return "The request precondition was not met. Update the precondition and retry." + requestReference(request.RequestID)
	case "NotImplemented":
		return "This storage operation is not supported. Use a supported operation and retry." + requestReference(request.RequestID)
	case "InvalidPart":
		return "The multipart part is invalid. Verify the part number and ETag, then retry." + requestReference(request.RequestID)
	case "InvalidPartSize":
		return "The multipart part size is invalid. Use one consistent part size except for the final part, then retry." + requestReference(request.RequestID)
	case "SlowDown":
		// ADR-008 messages are a deliberate client contract: they explain the
		// part-size rule and are already free of implementation details.
		if strings.TrimSpace(detail) != "" {
			return detail
		}
		return "The upload must wait before retrying this part. Retry the request." + requestReference(request.RequestID)
	}

	if request.Status >= 500 {
		return "The server could not complete this request. Retry it and include the request ID if the failure continues." + requestReference(request.RequestID)
	}
	return "The request could not be completed. Correct the request and retry." + requestReference(request.RequestID)
}

func requestReference(requestID string) string {
	if requestID == "" {
		return " Include the request ID from the response when reporting this failure."
	}
	return fmt.Sprintf(" Request ID: %s.", requestID)
}
