package config

import (
	"testing"
)

func TestParseACL(t *testing.T) {
	tests := []struct {
		name        string
		aclStr      string
		expectCount int
		expectError bool
		checkFunc   func([]ACLEntry) bool
	}{
		{
			name:        "empty string - returns nil",
			aclStr:      "",
			expectCount: 0, // parseACL returns nil for empty string
	 },
		{
			name:        "single bucket with wildcard prefix",
			aclStr:      "my-bucket:*",
			expectCount: 1,
		 checkFunc: func(acls []ACLEntry) bool {
			 return acls[0].Bucket == "my-bucket" && acls[0].Prefix == ""
            },
        },
        {
            name:        "single bucket with specific prefix",
            aclStr:      "my-bucket:data/",
            expectCount: 1,
            checkFunc: func(acls []ACLEntry) bool {
                return acls[0].Bucket == "my-bucket" && acls[0].Prefix == "data/"
            },
        },
        {
            name:        "multiple entries",
            aclStr:      "bucket-a:prefix-a/,bucket-b:prefix-b/",
            expectCount: 2,
            checkFunc: func(acls []ACLEntry) bool {
                return acls[0].Bucket == "bucket-a" && acls[0].Prefix == "prefix-a/" &&
                    acls[1].Bucket == "bucket-b" && acls[1].Prefix == "prefix-b/"
            },
        },
        {
            name:        "wildcard bucket",
            aclStr:      "*:public/",
            expectCount: 1,
            checkFunc: func(acls []ACLEntry) bool {
                return acls[0].Bucket == "*" && acls[0].Prefix == "public/"
            },
        },
        {
            name:        "invalid format - missing colon",
            aclStr:      "bucket-only",
            expectError: true,
        },
        {
            name:        "invalid format - empty bucket",
            aclStr:      ":prefix/",
            expectError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            acls, err := parseACL(tt.aclStr)

            if tt.name == "empty string - returns nil" {
                // parseACL returns nil for empty string (meaning no ACLs)
                if acls != nil {
                    t.Errorf("expected nil for empty string, got %v", acls)
                }
                return
            }

            if tt.expectError {
                if err == nil {
                    t.Error("expected error, got nil")
                }
                return
            }

            if err != nil {
                t.Errorf("unexpected error: %v", err)
                return
            }

            if len(acls) != tt.expectCount {
                t.Errorf("expected %d ACLs, got %d", tt.expectCount, len(acls))
            }

            if tt.checkFunc != nil && !tt.checkFunc(acls) {
                t.Error("ACL check function failed")
            }
        })
    }
}

func TestParseKeyRoutes(t *testing.T) {
    tests := []struct {
        name        string
        routesStr   string
        expectCount int
        expectError bool
        checkFunc   func([]KeyRoute) bool
    }{
        {
            name:        "empty string",
            routesStr:   "",
            expectCount: 0,
        },
        {
            name:        "single route",
            routesStr:   "data/=sensitive",
            expectCount: 1,
            checkFunc: func(routes []KeyRoute) bool {
                return routes[0].Prefix == "data/" && routes[0].KeyName == "sensitive"
            },
        },
        {
            name:        "multiple routes",
            routesStr:   "data/pii/*=sensitive,archive/*=archive,*=default",
            expectCount: 3,
            checkFunc: func(routes []KeyRoute) bool {
                return routes[0].Prefix == "data/pii/*" && routes[0].KeyName == "sensitive" &&
                    routes[1].Prefix == "archive/*" && routes[1].KeyName == "archive" &&
                    routes[2].Prefix == "" && routes[2].KeyName == "default"
            },
        },
        {
            name:        "invalid format",
            routesStr:   "invalid-format",
            expectError: true,
        },
        {
            name:        "empty prefix",
            routesStr:   "=keyname",
            expectError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            routes, err := parseKeyRoutes(tt.routesStr)

            if tt.expectError {
                if err == nil {
                    t.Error("expected error, got nil")
                }
                return
            }

            if err != nil {
                t.Errorf("unexpected error: %v", err)
                return
            }

            if len(routes) != tt.expectCount {
                t.Errorf("expected %d routes, got %d", tt.expectCount, len(routes))
            }

            if tt.checkFunc != nil && !tt.checkFunc(routes) {
                t.Error("route check function failed")
            }
        })
    }
}
