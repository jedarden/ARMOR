// boto3 compatibility leg — botocore's real SigV4 signer against the real
// ARMOR request pipeline, via the driver in testdata/boto3_leg.py.
//
// The production-image boto3 gate (tests/test_s3_basic_operations.py, run by
// `make compat-boto3`) already covers the full documented API against a built
// image in endpoint mode. This leg keeps boto3 inside the Go suite's client
// matrix so the same coverage shape runs wherever the suite runs — in-process
// locally, or against the same running deployment in ARMOR_COMPAT_ENDPOINT
// mode — and so a boto3 regression fails this gate even if the python
// make-target leg is dropped from a caller's invocation.
package awsclicompat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBoto3_ClientLeg(t *testing.T) {
	requirePythonModule(t, "boto3", "install with `pip install boto3` (the gate pins boto3 1.35.36)")

	endpoint := startArmorServer(t)

	accessKey, secretKey := compatCredentials(t)

	// The driver lives beside this test in testdata/; `go test` runs with the
	// package directory as its working directory.
	driver := filepath.Join("testdata", "boto3_leg.py")
	env := mergeEnv(os.Environ(), map[string]string{
		"AWS_ACCESS_KEY_ID":         accessKey,
		"AWS_SECRET_ACCESS_KEY":     secretKey,
		"AWS_DEFAULT_REGION":        testRegion,
		"AWS_EC2_METADATA_DISABLED": "true",
	})
	out := mustRun(t, "python3", env, driver, endpoint, compatBucket(t), testRegion)
	t.Log(out)
}
